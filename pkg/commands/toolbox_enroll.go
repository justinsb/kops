/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package commands

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/v1alpha2"
	"k8s.io/kops/pkg/apis/nodeup"
	"k8s.io/kops/pkg/assets"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/commands/commandutils"
	"k8s.io/kops/pkg/featureflag"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/model/resources"
	"k8s.io/kops/pkg/nodemodel"
	"k8s.io/kops/pkg/wellknownservices"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kops/util/pkg/vfs"
)

// TODO: How do control plane machines get their host registered (and do we care?)
// TODO: need to add kops-controller.internal.<cluster>.k8s.local DNS to nodes
// TODO: Why is /etc/hosts blank?

// TODO: Merge justinsb/metal_enroll

// kubectl apply --server-side -f - <<EOF
// apiVersion: rbac.authorization.k8s.io/v1
// kind: ClusterRoleBinding
// metadata:
//   name: kops-controller:pki-verifier
// roleRef:
//   apiGroup: rbac.authorization.k8s.io
//   kind: ClusterRole
//   name: kops-controller:pki-verifier
// subjects:
// - apiGroup: rbac.authorization.k8s.io
//   kind: User
//   name: system:serviceaccount:kube-system:kops-controller
// ---
// apiVersion: rbac.authorization.k8s.io/v1
// kind: ClusterRole
// metadata:
//   name: kops-controller:pki-verifier
// rules:
// - apiGroups:
//   - "kops.k8s.io"
//   resources:
//   - hosts
//   verbs:
//   - get
//   - list
//   - watch
// EOF

type ToolboxEnrollOptions struct {
	ClusterName   string
	InstanceGroup string

	Host string

	SSHUser string
	SSHPort int

	PodCIDRs []string
}

func (o *ToolboxEnrollOptions) InitDefaults() {
	o.SSHUser = "root"
	o.SSHPort = 22
}

func RunToolboxEnroll(ctx context.Context, f commandutils.Factory, out io.Writer, options *ToolboxEnrollOptions) error {
	if !featureflag.Metal.Enabled() {
		return fmt.Errorf("bare-metal support requires the Metal feature flag to be enabled")
	}
	if options.ClusterName == "" {
		return fmt.Errorf("cluster is required")
	}
	if options.InstanceGroup == "" {
		return fmt.Errorf("instance-group is required")
	}
	clientset, err := f.KopsClient()
	if err != nil {
		return err
	}

	cluster, err := clientset.GetCluster(ctx, options.ClusterName)
	if err != nil {
		return err
	}

	if cluster == nil {
		return fmt.Errorf("cluster not found %q", options.ClusterName)
	}

	channel, err := cloudup.ChannelForCluster(clientset.VFSContext(), cluster)
	if err != nil {
		return fmt.Errorf("getting channel for cluster %q: %w", options.ClusterName, err)
	}

	cloud, err := cloudup.BuildCloud(cluster)
	if err != nil {
		return fmt.Errorf("building cloud: %w", err)
	}

	instanceGroupList, err := clientset.InstanceGroupsFor(cluster).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	apply := &cloudup.ApplyClusterCmd{
		Cloud:      cloud,
		Cluster:    cluster,
		Clientset:  clientset,
		DryRun:     true,
		TargetName: cloudup.TargetDryRun,
	}
	applyResults, err := apply.Run(ctx)
	if err != nil {
		return fmt.Errorf("error during apply: %w", err)
	}

	// assetBuilder := assets.NewAssetBuilder(clientset.VFSContext(), cluster.Spec.Assets, cluster.Spec.KubernetesVersion, false)
	assetBuilder := applyResults.AssetBuilder
	var fullInstanceGroup *kops.InstanceGroup
	var fullCluster *kops.Cluster
	{
		var instanceGroups []*kops.InstanceGroup
		for i := range instanceGroupList.Items {
			instanceGroup := &instanceGroupList.Items[i]
			instanceGroups = append(instanceGroups, instanceGroup)
		}

		populatedCluster, err := cloudup.PopulateClusterSpec(ctx, clientset, cluster, instanceGroups, cloud, assetBuilder)
		if err != nil {
			return fmt.Errorf("building full cluster spec: %w", err)
		}
		fullCluster = populatedCluster

		// Build full IG spec to ensure we end up with a valid IG
		for _, ig := range instanceGroups {
			if ig.Name != options.InstanceGroup {
				continue
			}
			populated, err := cloudup.PopulateInstanceGroupSpec(fullCluster, ig, cloud, channel)
			if err != nil {
				return err
			}
			fullInstanceGroup = populated
		}

		if fullInstanceGroup == nil {
			return fmt.Errorf("instance group %q not found", options.InstanceGroup)
		}
	}

	klog.Infof("StaticAssets = %v", assetBuilder.StaticManifests)
	klog.Infof("StaticFiles = %v", assetBuilder.StaticFiles)
	wellKnownAddresses := make(model.WellKnownAddresses)

	{
		ingresses, err := cloud.GetApiIngressStatus(fullCluster)
		if err != nil {
			return fmt.Errorf("error getting ingress status: %v", err)
		}

		for _, ingress := range ingresses {
			// TODO: Do we need to support hostnames?
			// if ingress.Hostname != "" {
			// 	apiserverAdditionalIPs = append(apiserverAdditionalIPs, ingress.Hostname)
			// }
			if ingress.IP != "" {
				wellKnownAddresses[wellknownservices.KubeAPIServer] = append(wellKnownAddresses[wellknownservices.KubeAPIServer], ingress.IP)
				wellKnownAddresses[wellknownservices.KopsController] = append(wellKnownAddresses[wellknownservices.KopsController], ingress.IP)
			}
		}
	}

	if len(wellKnownAddresses[wellknownservices.KubeAPIServer]) == 0 {
		// TODO: Should we support DNS?
		return fmt.Errorf("unable to determine IP address for kube-apiserver")
	}

	for k := range wellKnownAddresses {
		sort.Strings(wellKnownAddresses[k])
	}

	bootstrapData, err := buildBootstrapData(ctx, clientset, fullCluster, fullInstanceGroup, assetBuilder, wellKnownAddresses)
	if err != nil {
		return fmt.Errorf("building bootstrap data: %w", err)
	}

	if options.Host != "" {
		// TODO: This is the pattern we use a lot, but should we try to access it directly?
		contextName := fullCluster.ObjectMeta.Name
		clientGetter := genericclioptions.NewConfigFlags(true)
		clientGetter.Context = &contextName

		restConfig, err := clientGetter.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("cannot load kubecfg settings for %q: %w", contextName, err)
		}

		if err := enrollHost(ctx, fullInstanceGroup, options, bootstrapData, restConfig); err != nil {
			return err
		}
	}
	return nil
}

func enrollHost(ctx context.Context, ig *kops.InstanceGroup, options *ToolboxEnrollOptions, bootstrapData *bootstrapData, restConfig *rest.Config) error {
	if len(options.PodCIDRs) == 0 {
		return fmt.Errorf("cannot enroll host without podCIDRs")
	}

	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		return fmt.Errorf("building kubernetes scheme: %w", err)
	}
	kubeClient, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("building kubernetes client: %w", err)
	}

	sudo := true
	if options.SSHUser == "root" {
		sudo = false
	}

	host, err := NewSSHHost(ctx, options.Host, options.SSHPort, options.SSHUser, sudo)
	if err != nil {
		return err
	}
	defer host.Close()

	hostObj := &v1alpha2.Host{}

	publicKeyPath := "/etc/kubernetes/kops/pki/machine/public.pem"

	publicKeyBytes, err := host.readFile(ctx, publicKeyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			publicKeyBytes = nil
		} else {
			return fmt.Errorf("error reading public key %q: %w", publicKeyPath, err)
		}
	}

	publicKeyBytes = bytes.TrimSpace(publicKeyBytes)
	if len(publicKeyBytes) == 0 {
		if _, err := host.runScript(ctx, scriptCreateKey, ExecOptions{Sudo: sudo, Echo: true}); err != nil {
			return err
		}

		b, err := host.readFile(ctx, publicKeyPath)
		if err != nil {
			return fmt.Errorf("error reading public key %q (after creation): %w", publicKeyPath, err)
		}
		publicKeyBytes = b
	}
	klog.V(2).Infof("public key is %s", string(publicKeyBytes))
	hostObj.Spec.PublicKey = string(publicKeyBytes)

	hostname, err := host.getHostname(ctx)
	if err != nil {
		return err
	}

	hostObj.Spec.InstanceGroup = ig.Name
	hostObj.Spec.Addresses = []string{options.Host}
	hostObj.Spec.PodCIDRs = options.PodCIDRs

	// ipLinks, err := host.getIPAddresses(ctx)
	// if err != nil {
	// 	return err
	// }

	// for _, link := range ipLinks {
	// 	for _, addr := range link.AddressInfo {
	// 		s := fmt.Sprintf("%s/%d", addr.Local, addr.PrefixLength)
	// 		ip, podCIDR, err := net.ParseCIDR(s)
	// 		if err != nil {
	// 			return fmt.Errorf("cannot parse cidr %q: %w", s, err)
	// 		}
	// 		if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
	// 			continue
	// 		}
	// 		if ip.To4() != nil {
	// 			continue
	// 		}
	// 		hostObj.Spec.PodCIDRs = append(hostObj.Spec.PodCIDRs, podCIDR.String())
	// 	}
	// }

	isControlPlane := ig.HasAPIServer()
	// Pre-create, so node bootstrap works
	if !isControlPlane {
		if err := createHost(ctx, hostname, hostObj, kubeClient); err != nil {
			return err
		}
	}

	for k, v := range bootstrapData.configFiles {
		if err := host.writeFile(ctx, k, bytes.NewReader(v)); err != nil {
			return fmt.Errorf("writing file %q over SSH: %w", k, err)
		}
	}
	if len(bootstrapData.nodeupScript) != 0 {
		if _, err := host.runScript(ctx, string(bootstrapData.nodeupScript), ExecOptions{Sudo: sudo, Echo: true}); err != nil {
			return err
		}
	}

	// Not needed for bootstrap, create after kube-apiserver is (hopefully) running
	if isControlPlane {
		if err := createHost(ctx, hostname, hostObj, kubeClient); err != nil {
			return err
		}
	}

	return nil
}

func createHost(ctx context.Context, nodeName string, host *v1alpha2.Host, c client.Client) error {
	host.Namespace = "kops-system"
	host.Name = nodeName

	// if err := client.Create(ctx, host); err != nil {
	// 	return fmt.Errorf("failed to create host %s/%s: %w", host.Namespace, host.Name, err)
	// }

	host.SetGroupVersionKind(v1alpha2.SchemeGroupVersion.WithKind("Host"))

	if err := c.Patch(ctx, host, client.Apply, client.FieldOwner("kops:enroll")); err != nil {
		return fmt.Errorf("failed to create host %s/%s: %w", host.Namespace, host.Name, err)
	}

	return nil
}

const scriptCreateKey = `
#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

set -x

DIR=/etc/kubernetes/kops/pki/machine/
mkdir -p ${DIR}

if [[ ! -f "${DIR}/private.pem" ]]; then
  openssl ecparam -name prime256v1 -genkey -noout -out "${DIR}/private.pem"
fi

if [[ ! -f "${DIR}/public.pem" ]]; then
  openssl ec -in "${DIR}/private.pem" -pubout -out "${DIR}/public.pem"
fi
`

// SSHHost is a wrapper around an SSH connection to a host machine.
type SSHHost struct {
	hostname  string
	sshClient *ssh.Client
	sudo      bool
}

// Close closes the connection.
func (s *SSHHost) Close() error {
	if s.sshClient != nil {
		if err := s.sshClient.Close(); err != nil {
			return err
		}
		s.sshClient = nil
	}
	return nil
}

// NewSSHHost creates a new SSHHost.
func NewSSHHost(ctx context.Context, host string, sshPort int, sshUser string, sudo bool) (*SSHHost, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("cannot connect to SSH agent; SSH_AUTH_SOCK env variable not set")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent with SSH_AUTH_SOCK %q: %w", socket, err)
	}

	agentClient := agent.NewClient(conn)

	sshConfig := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			klog.Warningf("accepting SSH key %v for %q", key, hostname)
			return nil
		},
		Auth: []ssh.AuthMethod{
			// Use a callback rather than PublicKeys so we only consult the
			// agent once the remote server wants it.
			ssh.PublicKeysCallback(agentClient.Signers),
		},
		User: sshUser,
	}
	sshClient, err := ssh.Dial("tcp", host+":"+strconv.Itoa(sshPort), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to SSH to %q (with user %q): %w", host, sshUser, err)
	}
	return &SSHHost{
		hostname:  host,
		sshClient: sshClient,
		sudo:      sudo,
	}, nil
}

func (s *SSHHost) readFile(ctx context.Context, path string) ([]byte, error) {
	p := vfs.NewSSHPath(s.sshClient, s.hostname, path, s.sudo)

	return p.ReadFile(ctx)
}

func (s *SSHHost) writeFile(ctx context.Context, path string, data io.ReadSeeker) error {
	p := vfs.NewSSHPath(s.sshClient, s.hostname, path, s.sudo)

	return p.WriteFile(ctx, data, nil)
}

func (s *SSHHost) runScript(ctx context.Context, script string, options ExecOptions) (*CommandOutput, error) {
	var tempDir string
	{
		b := make([]byte, 32)
		if _, err := cryptorand.Read(b); err != nil {
			return nil, fmt.Errorf("error getting random data: %w", err)
		}
		tempDir = path.Join("/tmp", hex.EncodeToString(b))
	}

	scriptPath := path.Join(tempDir, "script.sh")

	p := vfs.NewSSHPath(s.sshClient, s.hostname, scriptPath, s.sudo)

	defer func() {
		if _, err := s.runCommand(ctx, "rm -rf "+tempDir, ExecOptions{Sudo: s.sudo, Echo: false}); err != nil {
			klog.Warningf("error cleaning up temp directory %q: %v", tempDir, err)
		}
	}()

	if err := p.WriteFile(ctx, bytes.NewReader([]byte(script)), nil); err != nil {
		return nil, fmt.Errorf("error writing script to SSH target: %w", err)
	}

	scriptCommand := "/bin/bash " + scriptPath
	return s.runCommand(ctx, scriptCommand, options)
}

// CommandOutput holds the results of running a command.
type CommandOutput struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

// ExecOptions holds options for running a command remotely.
type ExecOptions struct {
	Sudo bool
	Echo bool
}

func (s *SSHHost) runCommand(ctx context.Context, command string, options ExecOptions) (*CommandOutput, error) {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to start SSH session: %w", err)
	}
	defer session.Close()

	output := &CommandOutput{}

	session.Stdout = &output.Stdout
	session.Stderr = &output.Stderr

	if options.Echo {
		session.Stdout = io.MultiWriter(os.Stdout, session.Stdout)
		session.Stderr = io.MultiWriter(os.Stderr, session.Stderr)
	}
	if options.Sudo {
		command = "sudo " + command
	}
	if err := session.Run(command); err != nil {
		return output, fmt.Errorf("error running command %q: %w", command, err)
	}
	return output, nil
}

// getHostname gets the hostname of the SSH target.
// This is used as the node name when registering the node.
func (s *SSHHost) getHostname(ctx context.Context) (string, error) {
	output, err := s.runCommand(ctx, "hostname", ExecOptions{Sudo: false, Echo: true})
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	hostname := output.Stdout.String()
	hostname = strings.TrimSpace(hostname)
	if len(hostname) == 0 {
		return "", fmt.Errorf("hostname was empty")
	}
	return hostname, nil
}

// getIPAddresses gets and parses the `ip addr list` output.
// This is used to compute the pod CIDRs.
func (s *SSHHost) getIPAddresses(ctx context.Context) ([]ipLinkInfo, error) {
	output, err := s.runCommand(ctx, "ip -json addr list", ExecOptions{Sudo: false, Echo: true})
	if err != nil {
		return nil, fmt.Errorf("running command 'ip -json addr list': %w", err)
	}

	j := output.Stdout.Bytes()
	var links []ipLinkInfo
	if err := json.Unmarshal(j, &links); err != nil {
		return nil, fmt.Errorf("parsing output of 'ip -json addr list': %w", err)
	}
	return links, nil
}

type ipLinkInfo struct {
	IFIndex     int             `json:"ifindex"`
	IFName      string          `json:"ifname"`
	AddressInfo []ipAddressInfo `json:"addr_info"`
}

type ipAddressInfo struct {
	Family       string `json:"family"`
	Local        string `json:"local"`
	PrefixLength int    `json:"prefixlen"`
	Scope        string `json:"scope"`
}
type bootstrapData struct {
	nodeupScript []byte
	configFiles  map[string][]byte
}

func buildBootstrapData(ctx context.Context, clientset simple.Clientset, cluster *kops.Cluster, ig *kops.InstanceGroup, assetBuilder *assets.AssetBuilder, wellknownAddresses model.WellKnownAddresses) (*bootstrapData, error) {
	bootstrapData := &bootstrapData{}

	// if cluster.Spec.KubeAPIServer == nil {
	// 	cluster.Spec.KubeAPIServer = &kops.KubeAPIServerConfig{}
	// }

	// getAssets := false
	// assetBuilder := assets.NewAssetBuilder(vfsContext, cluster.Spec.Assets, cluster.Spec.KubernetesVersion, getAssets)

	encryptionConfigSecretHash := ""
	// TODO: Support encryption config?
	// if fi.ValueOf(c.Cluster.Spec.EncryptionConfig) {
	// 	secret, err := secretStore.FindSecret("encryptionconfig")
	// 	if err != nil {
	// 		return fmt.Errorf("could not load encryptionconfig secret: %v", err)
	// 	}
	// 	if secret == nil {
	// 		fmt.Println("")
	// 		fmt.Println("You have encryptionConfig enabled, but no encryptionconfig secret has been set.")
	// 		fmt.Println("See `kops create secret encryptionconfig -h` and https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/")
	// 		return fmt.Errorf("could not find encryptionconfig secret")
	// 	}
	// 	hashBytes := sha256.Sum256(secret.Data)
	// 	encryptionConfigSecretHash = base64.URLEncoding.EncodeToString(hashBytes[:])
	// }

	// nodeUpAssets := make(map[architectures.Architecture]*assets.MirroredAsset)
	// for _, arch := range architectures.GetSupported() {
	// 	asset, err := wellknownassets.NodeUpAsset(assetBuilder, arch)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	nodeUpAssets[arch] = asset
	// }

	fileAssets := &nodemodel.FileAssets{Cluster: cluster}
	if err := fileAssets.AddFileAssets(assetBuilder); err != nil {
		return nil, err
	}

	// assets := make(map[architectures.Architecture][]*assets.MirroredAsset)
	configBuilder, err := nodemodel.NewNodeUpConfigBuilder(cluster, assetBuilder, fileAssets.Assets, encryptionConfigSecretHash)
	if err != nil {
		return nil, err
	}

	keysets := make(map[string]*fi.Keyset)

	keystore, err := clientset.KeyStore(cluster)
	if err != nil {
		return nil, err
	}

	keyNames := []string{"kubernetes-ca", "etcd-clients-ca"}
	if ig.HasAPIServer() {
		keyNames = append(keyNames, "etcd-clients-ca")
	}

	for _, etcdCluster := range cluster.Spec.EtcdClusters {
		k := etcdCluster.Name
		keyNames = append(keyNames, "etcd-manager-ca-"+k, "etcd-peers-ca-"+k)
		if k != "events" && k != "main" {
			keyNames = append(keyNames, "etcd-clients-ca-"+k)
		}
	}

	if ig.HasAPIServer() {
		keyNames = append(keyNames, "apiserver-aggregator-ca", "service-account", "etcd-clients-ca")
	}

	if ig.IsBastion() {
		keyNames = nil
	}

	for _, keyName := range keyNames {
		keyset, err := keystore.FindKeyset(ctx, keyName)
		if err != nil {
			return nil, fmt.Errorf("getting keyset %q: %w", keyName, err)
		}

		if keyset == nil {
			return nil, fmt.Errorf("failed to find keyset %q", keyName)
		}

		keysets[keyName] = keyset
	}

	nodeupConfig, bootConfig, err := configBuilder.BuildConfig(ig, wellknownAddresses, keysets)
	if err != nil {
		return nil, err
	}

	configBase, err := clientset.ConfigBaseFor(cluster)
	if err != nil {
		return nil, err
	}

	configFiles, err := rewriteFiles(ctx, clientset, configBase, nodeupConfig)
	if err != nil {
		return nil, err
	}
	bootstrapData.configFiles = configFiles

	bootConfig.CloudProvider = "metal"

	bootConfig.ConfigBase = fi.PtrTo("file:///opt/kops/conf")

	if bootConfig.InstanceGroupRole == kops.InstanceGroupRoleControlPlane {
		nodeupConfigBytes, err := yaml.Marshal(nodeupConfig)
		if err != nil {
			return nil, fmt.Errorf("error converting nodeup config to yaml: %w", err)
		}
		// sum256 := sha256.Sum256(configData)
		// bootConfig.NodeupConfigHash = base64.StdEncoding.EncodeToString(sum256[:])
		// b.nodeupConfig.Resource = fi.NewBytesResource(configData)

		p := filepath.Join("/opt/kops/conf", "igconfig", bootConfig.InstanceGroupRole.ToLowerString(), ig.Name, "nodeupconfig.yaml")
		bootstrapData.configFiles[p] = nodeupConfigBytes
	}

	// TODO: Should we / can we specify the node config hash?
	// configData, err := utils.YamlMarshal(config)
	// if err != nil {
	// 	return nil, fmt.Errorf("error converting nodeup config to yaml: %v", err)
	// }
	// sum256 := sha256.Sum256(configData)
	// bootConfig.NodeupConfigHash = base64.StdEncoding.EncodeToString(sum256[:])

	var nodeupScript resources.NodeUpScript
	nodeupScript.NodeUpAssets = fileAssets.NodeUpAssets
	nodeupScript.BootConfig = bootConfig

	{
		nodeupScript.EnvironmentVariables = func() (string, error) {
			env := make(map[string]string)

			// TODO: Support the full set of environment variables?
			// env, err := b.buildEnvironmentVariables()
			// if err != nil {
			// 	return "", err
			// }

			// Sort keys to have a stable sequence of "export xx=xxx"" statements
			var keys []string
			for k := range env {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			var b bytes.Buffer
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("export %s=%s\n", k, env[k]))
			}
			return b.String(), nil
		}

		nodeupScript.ProxyEnv = func() (string, error) {
			return "", nil
			// TODO: Support proxy?
			// return b.createProxyEnv(cluster.Spec.Networking.EgressProxy)
		}
	}

	// TODO: Support sysctls?
	// By setting some sysctls early, we avoid broken configurations that prevent nodeup download.
	// See https://github.com/kubernetes/kops/issues/10206 for details.
	// nodeupScript.SetSysctls = setSysctls()

	nodeupScript.CloudProvider = string(cluster.Spec.GetCloudProvider())

	nodeupScriptResource, err := nodeupScript.Build()
	if err != nil {
		return nil, err
	}

	nodeupScriptBytes, err := fi.ResourceAsBytes(nodeupScriptResource)
	if err != nil {
		return nil, err
	}
	bootstrapData.nodeupScript = nodeupScriptBytes

	return bootstrapData, nil
}

func rewriteFiles(ctx context.Context, clientset simple.Clientset, configBase vfs.Path, nodeupConfig *nodeup.Config) (map[string][]byte, error) {
	// vfsContext := clientset.VFSContext()

	configFiles := make(map[string][]byte)

	buildPath := func(vfsPath string) (vfs.Path, error) {
		base := configBase.Path()
		if !strings.HasSuffix(base, "/") {
			base += "/"
		}
		if !strings.HasPrefix(vfsPath, base) {
			return nil, fmt.Errorf("path %q did not start with prefix %q", vfsPath, base)
		}
		relative := strings.TrimPrefix(vfsPath, base)
		return configBase.Join(relative), nil
	}

	rewriteFile := func(srcPath vfs.Path) (string, error) {
		b, err := srcPath.ReadFile(ctx)
		if err != nil {
			return "", fmt.Errorf("reading vfs file %q: %w", srcPath.Path(), err)
		}

		p := filepath.Join("/opt/kops/conf", strings.ReplaceAll(srcPath.Path(), "://", "/"))
		configFiles[p] = b
		return "file://" + p, nil
	}

	rewriteRelativePath := func(relativePath string) error {
		srcPath := configBase.Join(relativePath)

		b, err := srcPath.ReadFile(ctx)
		if err != nil {
			return fmt.Errorf("reading vfs file %q: %w", srcPath, err)
		}

		p := filepath.Join("/opt/kops/conf", relativePath)
		configFiles[p] = b
		return nil
	}

	rewriteFileTree := func(vfsPath string) (string, error) {
		srcPath, err := buildPath(vfsPath) //vfsContext.BuildVfsPath(vfsPath)
		if err != nil {
			return "", fmt.Errorf("error building vfs path %q: %w", vfsPath, err)
		}

		srcFiles, err := srcPath.ReadTree(ctx)
		if err != nil {
			return "", fmt.Errorf("reading vfs tree %q: %w", vfsPath, err)
		}

		klog.Infof("ReadTree %v => %v", vfsPath, srcFiles)

		for _, srcFile := range srcFiles {
			b, err := srcFile.ReadFile(ctx)
			if err != nil {
				return "", fmt.Errorf("reading vfs file %q: %w", srcFile.Path(), err)
			}
			p := filepath.Join("/opt/kops/conf", strings.ReplaceAll(srcFile.Path(), "://", "/"))
			klog.Infof("ReadTree %v :: %v", srcFile, p)
			configFiles[p] = b
		}

		rootPath := filepath.Join("/opt/kops/conf", strings.ReplaceAll(srcPath.Path(), "://", "/"))
		return "file://" + rootPath, nil
	}

	for i, srcPath := range nodeupConfig.EtcdManifests {
		srcVFS, err := buildPath(srcPath)
		if err != nil {
			return nil, err
		}
		p, err := rewriteFile(srcVFS)
		if err != nil {
			return nil, err
		}
		nodeupConfig.EtcdManifests[i] = p
	}

	parentPath := func(s string) string {
		index := strings.LastIndex(s, "/")
		return s[:index+1]
	}

	for i, srcPath := range nodeupConfig.Channels {
		srcVFS, err := buildPath(srcPath)
		if err != nil {
			return nil, err
		}
		p, err := rewriteFile(srcVFS)
		if err != nil {
			return nil, err
		}

		parent := parentPath(srcPath)
		if _, err := rewriteFileTree(parent); err != nil {
			return nil, err
		}

		nodeupConfig.Channels[i] = p
	}

	for _, staticManifest := range nodeupConfig.StaticManifests {
		if err := rewriteRelativePath(staticManifest.Path); err != nil {
			return nil, err
		}
	}

	if nodeupConfig.ConfigStore != nil {
		p, err := rewriteFileTree(nodeupConfig.ConfigStore.Keypairs)
		if err != nil {
			return nil, err
		}
		nodeupConfig.ConfigStore.Keypairs = p
	}

	if nodeupConfig.ConfigStore != nil {
		p, err := rewriteFileTree(nodeupConfig.ConfigStore.Secrets)
		if err != nil {
			return nil, err
		}
		nodeupConfig.ConfigStore.Secrets = p
	}

	return configFiles, nil
}
