/*
Copyright 2018 The Kubernetes Authors.

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

package etcdmanager

import (
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/assets"
	"k8s.io/kops/pkg/flagbuilder"
	"k8s.io/kops/pkg/k8scodecs"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/fitasks"
	"k8s.io/kops/util/pkg/exec"
)

const metaFilename = "_etcd_backup.meta"

// EtcdManagerBuilder builds the manifest for the etcd-manager
type EtcdManagerBuilder struct {
	*model.KopsModelContext
	Lifecycle    *fi.Lifecycle
	AssetBuilder *assets.AssetBuilder
}

var _ fi.ModelBuilder = &EtcdManagerBuilder{}

func (b *EtcdManagerBuilder) Build(c *fi.ModelBuilderContext) error {
	for _, etcdCluster := range b.Cluster.Spec.EtcdClusters {
		if etcdCluster.Manager == nil {
			continue
		}

		name := etcdCluster.Name
		version := etcdCluster.Version

		backupStore := ""
		if etcdCluster.Backups != nil {
			backupStore = etcdCluster.Backups.BackupStore
		}
		if backupStore == "" {
			return fmt.Errorf("backupStore must be set for use with etcd-manager")
		}

		manifest, err := b.buildManifest(etcdCluster)
		if err != nil {
			return err
		}

		manifestYAML, err := k8scodecs.ToVersionedYaml(manifest)
		if err != nil {
			return fmt.Errorf("error marshalling manifest to yaml: %v", err)
		}

		c.AddTask(&fitasks.ManagedFile{
			Contents:  fi.WrapResource(fi.NewBytesResource(manifestYAML)),
			Lifecycle: b.Lifecycle,
			Location:  fi.String("manifests/etcd/" + name + ".yaml"),
			Name:      fi.String("manifests-etcdmanager-" + name),
		})

		info := &etcdClusterSpec{
			EtcdVersion: version,
			MemberCount: int32(len(etcdCluster.Members)),
		}

		d, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}

		c.AddTask(&fitasks.ManagedFile{
			Contents:  fi.WrapResource(fi.NewBytesResource(d)),
			Lifecycle: b.Lifecycle,
			// TODO: We need this to match the backup base (currently)
			Location: fi.String("backups/etcd/" + etcdCluster.Name + "/control/etcd-cluster-spec"),
			Name:     fi.String("etcd-cluster-spec-" + name),
		})
	}

	return nil
}

// type etcdBackupInfo struct {
// 	EtcdVersion string           `json:"etcd_version,omitempty"`
// 	Timestamp   int64            `json:"timestamp,omitempty"`
// 	IsCommand   bool             `json:"is_command,omitempty"`
// 	ClusterSpec *etcdClusterSpec `json:"cluster_spec,omitempty"`
// }

type etcdClusterSpec struct {
	MemberCount int32  `json:"member_count,omitempty"`
	EtcdVersion string `json:"etcd_version,omitempty"`
}

func (b *EtcdManagerBuilder) buildManifest(etcdCluster *kops.EtcdClusterSpec) (*v1.Pod, error) {
	if etcdCluster.Manager == nil {
		return nil, fmt.Errorf("manager not set for EtcdCluster")
	}

	return b.buildPod(etcdCluster)
}

// BuildEtcdManifest creates the pod spec, based on the etcd cluster
func (b *EtcdManagerBuilder) buildPod(etcdCluster *kops.EtcdClusterSpec) (*v1.Pod, error) {
	image := etcdCluster.Manager.Image
	{
		remapped, err := b.AssetBuilder.RemapImage(image)
		if err != nil {
			return nil, fmt.Errorf("unable to remap container %q: %v", image, err)
		} else {
			image = remapped
		}
	}

	isTLS := etcdCluster.EnableEtcdTLS

	address := "0.0.0.0"
	cpuRequest := resource.MustParse("100m")
	clientPort := 4001

	clusterName := "etcd-" + etcdCluster.Name
	// dataDirName := "data-" + etcdCluster.Name
	// electionTimeout:=   kubeBoot.EtcdElectionTimeout
	// heartbeatInterval:= kubeBoot.EtcdHeartbeatInterval
	// imageSource:=       kubeBoot.EtcdImageSource
	// peerCA:=            kubeBoot.PeerCA
	// 	peerCert:=          kubeBoot.PeerCert
	// 	peerKey:=           kubeBoot.PeerKey
	peerPort := 2380
	// podName:=           "etcd-server-" + clusterKey
	// 	tlsAuth:=           kubeBoot.TLSAuth
	// 	tlsCA:=             kubeBoot.TLSCA
	// tlsCert:=           kubeBoot.TLSCert
	// 	tlsKey:=            kubeBoot.TLSKey
	// 	volumeMountPath:=   v.Mountpoint
	backupStore := ""
	if etcdCluster.Backups != nil {
		backupStore = etcdCluster.Backups.BackupStore
	}

	podName := "etcd-manager-" + etcdCluster.Name

	// TODO: Use a socket file for the quarantine port
	quarantinedClientPort := 3994

	grpcPort := 3996

	// We used to build this through text files ... it turns out to just be more complicated than code!
	switch etcdCluster.Name {
	case "main":
		clusterName = "etcd"
		// dataDirName = "data"
		// podName = "etcd-server"
		cpuRequest = resource.MustParse("200m")

	case "events":
		clientPort = 4002
		peerPort = 2381
		grpcPort = 3997
		quarantinedClientPort = 3995

	default:
		return nil, fmt.Errorf("unknown etcd cluster key %q", etcdCluster.Name)
	}

	if backupStore == "" {
		return nil, fmt.Errorf("backupStore must be set for use with etcd-manager")
	}

	name := clusterName
	if !strings.HasPrefix(name, "etcd") {
		// For sanity, and to avoid collisions in directories / dns
		return nil, fmt.Errorf("unexpected name for etcd cluster (must start with etcd): %q", name)
	}
	logFile := "/var/log/" + name + ".log"

	// err := touchFile(pathFor(c.LogFile))
	// if err != nil {
	// 	return fmt.Errorf("error touching log-file %q: %v", c.LogFile, err)
	// }

	// clusterToken := "etcd-cluster-token-" + name

	// var nodes []*EtcdNode
	// for _, nodeName := range c.Spec.NodeNames {
	// 	name := name + "-" + nodeName
	// 	fqdn := k.BuildInternalDNSName(name)
	//
	// 	node := &EtcdNode{
	// 		Name:         name,
	// 		InternalName: fqdn,
	// 	}
	// 	nodes = append(nodes, node)
	// 	if nodeName == c.Spec.NodeName {
	// 		c.Me = node
	// 		if err = k.CreateInternalDNSNameRecord(fqdn); err != nil {
	// 			return fmt.Errorf("error mapping internal dns name for %q: %v", name, err)
	// 		}
	// 	}
	// }
	// c.Nodes = nodes
	//
	// if c.Me == nil {
	// 	return fmt.Errorf("my node name %s not found in cluster %v", c.Spec.NodeName, strings.Join(c.Spec.NodeNames, ","))
	// }

	// etcdVersion := etcdCluster.Version

	config := &EtcdManagerConfig{
		Containerized: true,
		PeerPort:      peerPort,
		ClusterName:   clusterName,
		BackupStore:   backupStore,
		GrpcPort:      grpcPort,
		Address:       address,
	}

	var envs []v1.EnvVar

	{
		// @check if we are using TLS
		scheme := "http"
		if isTLS {
			scheme = "https"
		}

		config.ClientUrls = fmt.Sprintf("%s://0.0.0.0:%d", scheme, clientPort)
		config.QuarantineClientUrls = fmt.Sprintf("%s://0.0.0.0:%d", scheme, quarantinedClientPort)

		// add the default setting for masters - http or https
		// envs = append(envs, []v1.EnvVar{
		// {Name: "ETCD_NAME", Value: c.Me.Name},
		// {Name: "ETCD_DATA_DIR", Value: "/var/etcd/" + dataDirName},
		// {Name: "ETCD_LISTEN_PEER_URLS", Value: fmt.Sprintf("%s://0.0.0.0:%d", scheme, peerPort)},
		// {Name: "ETCD_LISTEN_CLIENT_URLS", Value: fmt.Sprintf("%s://0.0.0.0:%d", scheme, clientPort)},
		// {Name: "ETCD_ADVERTISE_CLIENT_URLS", Value: fmt.Sprintf("%s://%s:%d", scheme, c.Me.InternalName, c.ClientPort)},
		// {Name: "ETCD_INITIAL_ADVERTISE_PEER_URLS", Value: fmt.Sprintf("%s://%s:%d", scheme, c.Me.InternalName, c.PeerPort)},
		// {Name: "ETCD_INITIAL_CLUSTER_STATE", Value: "new"},
		// {Name: "ETCD_INITIAL_CLUSTER_TOKEN", Value: c.ClusterToken}}...)

		// TODO: We need to wire these into the etcd-manager spec
		// // add timeout/heartbeat settings
		if etcdCluster.LeaderElectionTimeout != nil {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_ELECTION_TIMEOUT", Value: convEtcdSettingsToMs(etcdClusterSpec.LeaderElectionTimeout)})
			return nil, fmt.Errorf("LeaderElectionTimeout not supported by etcd-manager")
		}
		if etcdCluster.HeartbeatInterval != nil {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_HEARTBEAT_INTERVAL", Value: convEtcdSettingsToMs(etcdClusterSpec.HeartbeatInterval)})
			return nil, fmt.Errorf("HeartbeatInterval not supported by etcd-manager")
		}

		if isTLS {
			return nil, fmt.Errorf("TLS not supported for etcd-manager")

			// // check if we are using tls and add the options to protokube
			// if t.UseEtcdTLS() {
			// 	// etcd-manager should read from the keystore directly
			// 	return nil, fmt.Errorf("TODO: TLS")
			// 	// f.PeerTLSCaFile = s(filepath.Join(t.PathSrvKubernetes(), "ca.crt"))
			// 	// f.PeerTLSCertFile = s(filepath.Join(t.PathSrvKubernetes(), "etcd.pem"))
			// 	// f.PeerTLSKeyFile = s(filepath.Join(t.PathSrvKubernetes(), "etcd-key.pem"))
			// 	// f.TLSCAFile = s(filepath.Join(t.PathSrvKubernetes(), "ca.crt"))
			// 	// f.TLSCertFile = s(filepath.Join(t.PathSrvKubernetes(), "etcd.pem"))
			// 	// f.TLSKeyFile = s(filepath.Join(t.PathSrvKubernetes(), "etcd-key.pem"))
			// }
			// if t.UseTLSAuth() {
			// 	enableAuth := true
			// 	f.TLSAuth = b(enableAuth)
			// }

			// // @check if we are using peer certificates
			// if notEmpty(c.PeerCA) {
			// 	envs = append(envs, []v1.EnvVar{
			// 		{Name: "ETCD_PEER_TRUSTED_CA_FILE", Value: c.PeerCA}}...)
			// }
			// if notEmpty(c.PeerCert) {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_PEER_CERT_FILE", Value: c.PeerCert})
			// }
			// if notEmpty(c.PeerKey) {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_PEER_KEY_FILE", Value: c.PeerKey})
			// }
			// if notEmpty(c.TLSCA) {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_TRUSTED_CA_FILE", Value: c.TLSCA})
			// }
			// if notEmpty(c.TLSCert) {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_CERT_FILE", Value: c.TLSCert})
			// }
			// if notEmpty(c.TLSKey) {
			// 	envs = append(envs, v1.EnvVar{Name: "ETCD_KEY_FILE", Value: c.TLSKey})
			// }
			// if c.isTLS() {
			// 	if c.TLSAuth {
			// 		envs = append(envs, v1.EnvVar{Name: "ETCD_CLIENT_CERT_AUTH", Value: "true"})
			// 		envs = append(envs, v1.EnvVar{Name: "ETCD_PEER_CLIENT_CERT_AUTH", Value: "true"})
			// 	}
			// }
		}

		// // @step: generate the initial cluster
		// var hosts []string
		// for _, node := range c.Nodes {
		// 	hosts = append(hosts, node.Name+"="+fmt.Sprintf("%s://%s:%d", scheme, node.InternalName, c.PeerPort))
		// }
		// envs = append(envs, v1.EnvVar{Name: "ETCD_INITIAL_CLUSTER", Value: strings.Join(hosts, ",")})
	}

	{
		switch kops.CloudProviderID(b.Cluster.Spec.CloudProvider) {
		case kops.CloudProviderAWS:
			config.VolumeProvider = "aws"

			config.VolumeTag = []string{
				fmt.Sprintf("kubernetes.io/cluster/%s=owned", b.Cluster.Name),
				awsup.TagNameEtcdClusterPrefix + etcdCluster.Name,
				awsup.TagNameRolePrefix + "master=1",
			}

		default:
			return nil, fmt.Errorf("CloudProvider %q not supported with etcd-manager", b.Cluster.Spec.CloudProvider)
		}
	}

	args, err := flagbuilder.BuildFlagsList(config)
	if err != nil {
		return nil, err
	}

	pod := &v1.Pod{}
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Name = podName
	pod.Namespace = "kube-system"
	pod.Labels = map[string]string{"k8s-app": podName}
	pod.Spec.HostNetwork = true

	{
		container := &v1.Container{
			Name:  "etcd-manager",
			Image: image,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: cpuRequest,
				},
			},
			Command: exec.WithTee("/etcd-manager", args, "/var/log/etcd.log"),
			Env:     envs,
		}

		// TODO: Reduce these permissions (they are needed for volume mounting)
		container.SecurityContext = &v1.SecurityContext{
			Privileged: fi.Bool(true),
		}

		// TODO: Liveness Probe
		// TODO: etcd-manager needs to health-check etcd

		// container.LivenessProbe = &v1.Probe{
		// 	InitialDelaySeconds: 15,
		// 	TimeoutSeconds:      15,
		// }
		// // ensure we have the correct probe schema
		// if c.isTLS() {
		// 	container.LivenessProbe.TCPSocket = &v1.TCPSocketAction{
		// 		Host: "127.0.0.1",
		// 		Port: intstr.FromInt(c.ClientPort),
		// 	}
		// } else {
		// 	container.LivenessProbe.HTTPGet = &v1.HTTPGetAction{
		// 		Host:   "127.0.0.1",
		// 		Port:   intstr.FromInt(c.ClientPort),
		// 		Path:   "/health",
		// 		Scheme: v1.URISchemeHTTP,
		// 	}
		// }

		// container.Ports = append(container.Ports, v1.ContainerPort{
		// 	Name:          "serverport",
		// 	ContainerPort: int32(c.PeerPort),
		// 	HostPort:      int32(c.PeerPort),
		// })
		// container.Ports = append(container.Ports, v1.ContainerPort{
		// 	Name:          "clientport",
		// 	ContainerPort: int32(c.ClientPort),
		// 	HostPort:      int32(c.ClientPort),
		// })

		// container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
		// 	Name:      "varetcdata",
		// 	MountPath: "/var/etcd/" + c.DataDirName,
		// 	ReadOnly:  false,
		// })

		// TODO: Use helper function here
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      "varlogetcd",
			MountPath: "/var/log/etcd.log",
			ReadOnly:  false,
		})

		// TODO: Would be nice to narrow this mount
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      "rootfs",
			MountPath: "/rootfs",
			ReadOnly:  false,
		})
		// // add the host path mount to the pod spec
		// pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		// 	Name: "varetcdata",
		// 	VolumeSource: v1.VolumeSource{
		// 		HostPath: &v1.HostPathVolumeSource{
		// 			Path: c.VolumeMountPath + "/var/etcd/" + c.DataDirName,
		// 		},
		// 	},
		// })
		hostPathFileOrCreate := v1.HostPathFileOrCreate
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "varlogetcd",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: logFile,
					Type: &hostPathFileOrCreate,
				},
			},
		})

		hostPathDirectory := v1.HostPathDirectory
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "rootfs",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/",
					Type: &hostPathDirectory,
				},
			},
		})

		kubemanifest.MapEtcdHosts(pod, container)
		// @check if tls is enabled and mount the directory. It might be worth considering
		// if we you use our own directory in /srv i.e /srv/etcd rather than the default /src/kubernetes
		if isTLS {
			return nil, fmt.Errorf("TLS not supported for etcd-manager")
			// for _, dirname := range buildCertificateDirectories(c) {
			// 	normalized := strings.Replace(dirname, "/", "", -1)
			// 	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			// 		Name: normalized,
			// 		VolumeSource: v1.VolumeSource{
			// 			HostPath: &v1.HostPathVolumeSource{
			// 				Path: dirname,
			// 			},
			// 		},
			// 	})
			// 	container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			// 		Name:      normalized,
			// 		MountPath: dirname,
			// 		ReadOnly:  true,
			// 	})
			// }
		}

		pod.Spec.Containers = append(pod.Spec.Containers, *container)
	}

	// if c.BackupStore != "" && c.BackupImage != "" {
	// 	backupContainer := buildEtcdBackupManagerContainer(c)
	// 	pod.Spec.Containers = append(pod.Spec.Containers, *backupContainer)
	// }

	kubemanifest.MarkPodAsCritical(pod)

	return pod, nil
}

// // buildCertificateDirectories generates a list of the base directories which the certificates are located
// // so we can map in as volumes. They will probably all be placed into /src/kubernetes, but just to make it
// // generic.
// func buildCertificateDirectories(c *EtcdCluster) []string {
// 	tracked := make(map[string]bool, 0)

// 	for _, x := range []string{c.TLSCA, c.TLSCert, c.TLSKey, c.PeerCA, c.PeerKey, c.PeerKey} {
// 		if x == "" || tracked[filepath.Dir(x)] == true {
// 			continue
// 		}
// 		tracked[filepath.Dir(x)] = true
// 	}

// 	var list []string
// 	for k := range tracked {
// 		list = append(list, k)
// 	}

// 	return list
// }

// EtcdManagerConfig are the flags for etcd-manager
type EtcdManagerConfig struct {
	Containerized bool `flag:"containerized"`

	Address              string   `flag:"address"`
	PeerPort             int      `flag:"peer-port"`
	GrpcPort             int      `flag:"grpc-port"`
	ClientUrls           string   `flag:"client-urls"`
	QuarantineClientUrls string   `flag:"quarantine-client-urls"`
	ClusterName          string   `flag:"cluster-name"`
	BackupStore          string   `flag:"backup-store"`
	DataDir              string   `flag:"data-dir"`
	VolumeProvider       string   `flag:"volume-provider"`
	VolumeTag            []string `flag:"volume-tag,repeat"`
}
