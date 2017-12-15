package baremetal

import (
	"k8s.io/kube-deploy/cluster-api-gcp/cloud"
	"fmt"
	clusterv1 "k8s.io/kube-deploy/cluster-api/api/cluster/v1alpha1"
	"k8s.io/kube-deploy/cluster-api/client"
	apierrors "k8s.io/kube-deploy/cluster-api-gcp/errors"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/kops/machine-controller/pkg/kopsproviderconfig"
	kopsproviderconfigv1 "k8s.io/kops/machine-controller/pkg/kopsproviderconfig/v1alpha1"
	"reflect"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/apis/kops"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kops/pkg/bundle"
	"k8s.io/kops/upup/pkg/kutil"
	"golang.org/x/crypto/ssh"
	"k8s.io/kops/util/pkg/vfs"
	"bytes"
	"io"
	"os"
)

type BaremetalActuator struct {
	sshCreds      SshCreds
	machineClient client.MachinesInterface

	clientset   simple.Clientset
	clusterName string

	// For parsing our provider object
	scheme       *runtime.Scheme
	codecFactory *serializer.CodecFactory
}

type SshCreds struct {
	user           string
	privateKeyPath string
}

// TODO: We should split MachineActuator into MachineActuator and ClusterActuator
var _ cloud.MachineActuator = &BaremetalActuator{}

func NewActuator(clientset simple.Clientset, clusterName string, sshUsername string, sshPrivateKeyPath string, machineClient client.MachinesInterface) (*BaremetalActuator, error) {
	scheme, codecFactory, err := kopsproviderconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}

	//// Only applicable if it's running inside machine controller pod.
	//var privateKeyPath, user string
	//if _, err := os.Stat("/etc/sshkeys/private"); err == nil {
	//	privateKeyPath = "/etc/sshkeys/private"
	//
	//	b, err := ioutil.ReadFile("/etc/sshkeys/user")
	//	if err == nil {
	//		user = string(b)
	//	} else {
	//		return nil, err
	//	}
	//}

	return &BaremetalActuator{
		//service:      service,
		scheme:       scheme,
		codecFactory: codecFactory,
		//kubeadmToken: kubeadmToken,
		clientset: clientset,
		clusterName: clusterName,
		sshCreds: SshCreds{
			user:           sshUsername,
			privateKeyPath: sshPrivateKeyPath,
		},
		machineClient: machineClient,
	}, nil
}

// Create the machine.
func (a *BaremetalActuator) Create(*clusterv1.Cluster, *clusterv1.Machine) error {
	return fmt.Errorf("BareMetalActuator::Create not implemented")
}

// Delete the machine.
func (a *BaremetalActuator) Delete(*clusterv1.Machine) error {
	return fmt.Errorf("BareMetalActuator::Delete not implemented")
}

// Update the machine to the provided definition.
func (a *BaremetalActuator) Update(c *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	// Before updating, do some basic validation of the object first.
	config, err := a.providerconfig(goalMachine.Spec.ProviderConfig)
	if err != nil {
		return a.handleMachineError(goalMachine,
			apierrors.InvalidMachineConfiguration("Cannot unmarshal providerConfig field: %v", err))
	}
	if verr := a.validateMachine(goalMachine, config); verr != nil {
		return a.handleMachineError(goalMachine, verr)
	}

	cluster, err := a.clientset.GetCluster(a.clusterName)
	if err != nil {
		return fmt.Errorf("error reading Cluster %q: %v", a.clusterName, err)
	}

	ig, err := a.clientset.InstanceGroupsFor(cluster).Get(config.InstanceGroup, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error reading InstanceGroup %q: %v", config.InstanceGroup, err)
	}
	if ig == nil {
		return fmt.Errorf("InstanceGroup %q not found", config.InstanceGroup)
	}

	status, err := a.instanceStatus(goalMachine)
	if err != nil {
		return err
	}

	currentMachine := (*clusterv1.Machine)(status)
	//if currentMachine == nil {
	//	instance, err := a.instanceIfExists(goalMachine)
	//	if err != nil {
	//		return err
	//	}
	//	if instance != nil && instance.Labels[BootstrapLabelKey] != "" {
	//		glog.Infof("Populating current state for bootstrap machine %v", goalMachine.ObjectMeta.Name)
	//		return a.updateAnnotations(goalMachine)
	//	} else {
	//		return fmt.Errorf("Cannot retrieve current state to update machine %v", goalMachine.ObjectMeta.Name)
	//	}
	//}

	if currentMachine != nil && !a.requiresUpdate(currentMachine, goalMachine) {
		return nil
	}

	glog.Infof("Doing an in-place upgrade for machine.\n")
	err = a.updateMachineInplace(cluster, ig, currentMachine, goalMachine)
	if err != nil {
		glog.Errorf("inplace update failed: %v", err)
		return err
	}
	err = a.updateInstanceStatus(goalMachine)
	return err
}

// Exists checks if the machine currently exists.
func (a *BaremetalActuator) Exists(machine *clusterv1.Machine) (bool, error) {
	glog.Infof("assuming machine exists for baremetal: %s", machine.Name)
	return true, nil
}

func (a *BaremetalActuator) GetIP(machine *clusterv1.Machine) (string, error) {
	return "", fmt.Errorf("BareMetalActuator::GetIP not implemented")
}

func (a *BaremetalActuator) GetKubeConfig(master *clusterv1.Machine) (string, error) {
	return "", fmt.Errorf("BareMetalActuator::GetKubeConfig not implemented")
}

// Create and start the machine controller. The list of initial
// machines don't have to be reconciled as part of this function, but
// are provided in case the function wants to refer to them (and their
// ProviderConfigs) to know how to configure the machine controller.
// Not idempotent.
func (a *BaremetalActuator) CreateMachineController(cluster *clusterv1.Cluster, initialMachines []*clusterv1.Machine) error {
	return fmt.Errorf("BareMetalActuator::CreateMachineController not implemented")
}

func (a *BaremetalActuator) PostDelete(cluster *clusterv1.Cluster, machines []*clusterv1.Machine) error {
	return fmt.Errorf("BareMetalActuator::PostDelete not implemented")
}

func (a *BaremetalActuator) updateMachineInplace(cluster *kops.Cluster, ig *kops.InstanceGroup, oldMachine *clusterv1.Machine, goalMachine *clusterv1.Machine) error {
	//if oldMachine.Spec.Versions.ControlPlane != newMachine.Spec.Versions.ControlPlane {
	//	// First pull off the latest kubeadm.
	//	cmd := "export KUBEADM_VERSION=$(curl -sSL https://dl.k8s.io/release/stable.txt); " +
	//		"curl -sSL https://dl.k8s.io/release/${KUBEADM_VERSION}/bin/linux/amd64/kubeadm | sudo tee /usr/bin/kubeadm > /dev/null; " +
	//		"sudo chmod a+rx /usr/bin/kubeadm"
	//	_, err := a.remoteSshCommand(newMachine, cmd)
	//	if err != nil {
	//		glog.Infof("remotesshcomand error: %v", err)
	//		return err
	//	}
	//
	//	// TODO: We might want to upgrade kubeadm if the target control plane version is newer.
	//	// Upgrade control plan.
	//	cmd = fmt.Sprintf("sudo kubeadm upgrade apply %s -y", "v"+newMachine.Spec.Versions.ControlPlane)
	//	_, err = a.remoteSshCommand(newMachine, cmd)
	//	if err != nil {
	//		glog.Infof("remotesshcomand error: %v", err)
	//		return err
	//	}
	//}
	//
	//// Upgrade kubelet.
	//if oldMachine.Spec.Versions.Kubelet != newMachine.Spec.Versions.Kubelet {
	//	cmd := fmt.Sprintf("sudo kubectl drain %s --kubeconfig /etc/kubernetes/admin.conf --ignore-daemonsets", newMachine.Name)
	//	// The errors are intentionally ignored as master has static pods.
	//	a.remoteSshCommand(newMachine, cmd)
	//	// Upgrade kubelet to desired version.
	//	cmd = fmt.Sprintf("sudo apt-get install kubelet=%s", newMachine.Spec.Versions.Kubelet+"-00")
	//	_, err := a.remoteSshCommand(newMachine, cmd)
	//	if err != nil {
	//		glog.Infof("remotesshcomand error: %v", err)
	//		return err
	//	}
	//	cmd = fmt.Sprintf("sudo kubectl uncordon %s --kubeconfig /etc/kubernetes/admin.conf", newMachine.Name)
	//	_, err = a.remoteSshCommand(newMachine, cmd)
	//	if err != nil {
	//		glog.Infof("remotesshcomand error: %v", err)
	//		return err
	//	}
	//}
	//
	//return nil

	//func
	//RunToolboxBundle(context
	//Factory, out
	//io.Writer, options * ToolboxBundleOptions, args []string) error{
	//	if len(args) == 0{
	//	return fmt.Errorf("Specify name of instance group for node")
	//}
	//	if len(args) != 1{
	//	return fmt.Errorf("Can only specify one instance group")
	//}

	//	if options.Target == ""{
	//	return fmt.Errorf("target is required")
	//}
	//	groupName := args[0]
	//
	//	cluster, err := rootCommand.Cluster()
	//	if err != nil{
	//	return err
	//}

	//clientset, err := context.Clientset()
	//if err != nil {
	//	return err
	//}

	config, err := a.providerconfig(goalMachine.Spec.ProviderConfig)
	if err != nil {
		return a.handleMachineError(goalMachine,
			apierrors.InvalidMachineConfiguration("Cannot unmarshal providerConfig field %q: %v", goalMachine.Spec.ProviderConfig, err))
	}

	if config.Target == "" {
		return a.handleMachineError(goalMachine,
			apierrors.InvalidMachineConfiguration("Target must be set for bare metal configuration"))
	}

	builder := bundle.Builder{
		Clientset: a.clientset,
	}
	bundleData, err := builder.Build(cluster, ig)
	if err != nil {
		return fmt.Errorf("error building bundle: %v", err)
	}

	glog.Infof("built bundle")

	nodeSSH := &kutil.NodeSSH{
		Hostname: config.Target,
	}
	nodeSSH.SSHConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	nodeSSH.SSHConfig.User = a.sshCreds.user
	if err := kutil.AddSSHIdentity(&nodeSSH.SSHConfig, a.sshCreds.privateKeyPath); err != nil {
		return err
	}

	glog.Infof("Making SSH connection to %s", nodeSSH.Hostname)

	sshClient, err := nodeSSH.GetSSHClient()
	if err != nil {
		return fmt.Errorf("error getting SSH client: %v", err)
	}

	if err := runSshCommand(sshClient, "sudo mkdir -p /etc/kubernetes/bootstrap"); err != nil {
		return err
	}

	root, err := nodeSSH.Root()
	if err != nil {
		return fmt.Errorf("error connecting to nodeSSH: %v", err)
	}
	for _, file := range bundleData.Files {
		sshAcl := &vfs.SSHAcl{
			Mode: file.Header.FileInfo().Mode(),
		}
		p := root.Join("etc", "kubernetes", "bootstrap", file.Header.Name)
		glog.Infof("writing %s", p)
		if err := p.WriteFile(file.Data, sshAcl); err != nil {
			return fmt.Errorf("error writing file %q: %v", file.Header.Name, err)
		}
	}

	if err := runSshCommand(sshClient, "sudo /etc/kubernetes/bootstrap/bootstrap.sh"); err != nil {
		return err
	}

	return nil
}

func runSshCommand(sshClient *ssh.Client, cmd string) error {
	s, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("error creating ssh session: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	s.Stdout = io.MultiWriter(&stdout, os.Stdout)
	s.Stderr = io.MultiWriter(&stderr, os.Stderr)

	glog.Infof("running %s", cmd)
	if err := s.Run(cmd); err != nil {
		return fmt.Errorf("error running %s: %v\nstdout: %s\nstderr: %s", cmd, err, stdout.String(), stderr.String())
	}

	glog.Infof("stdout: %s", stdout.String())
	glog.Infof("stderr: %s", stderr.String())
	return nil
}

//func writeToTar(files []*bundle.DataFile, bundlePath string) error {
//	f, err := os.Create(bundlePath)
//	if err != nil {
//	return fmt.Errorf("error creating output bundle file %q: %v", bundlePath, err)
//}
//	defer f.Close()
//
//	gw := gzip.NewWriter(f)
//	defer gw.Close()
//	tw := tar.NewWriter(gw)
//	defer tw.Close()
//
//	for _, file := range files {
//	if err := tw.WriteHeader(&file.Header); err != nil {
//	return fmt.Errorf("error writing tar file header: %v", err)
//}
//
//	if _, err := tw.Write(file.Data); err != nil {
//	return fmt.Errorf("error writing tar file data: %v", err)
//}
//}
//
//	return nil
//}

func (a *BaremetalActuator) validateMachine(machine *clusterv1.Machine, config *kopsproviderconfig.KopsProviderConfig) *apierrors.MachineError {
	if machine.Spec.Versions.Kubelet == "" {
		return apierrors.InvalidMachineConfiguration("spec.versions.kubelet can't be empty")
	}
	if machine.Spec.Versions.ContainerRuntime.Name != "docker" {
		return apierrors.InvalidMachineConfiguration("Only docker is supported")
	}
	if machine.Spec.Versions.ContainerRuntime.Version != "1.12.0" {
		return apierrors.InvalidMachineConfiguration("Only docker 1.12.0 is supported")
	}

	if config.InstanceGroup == "" {
		return apierrors.InvalidMachineConfiguration("InstanceGroup must be set for kops configuration")
	}

	return nil
}

func (a *BaremetalActuator) providerconfig(providerConfig string) (*kopsproviderconfig.KopsProviderConfig, error) {
	obj, gvk, err := a.codecFactory.UniversalDecoder().Decode([]byte(providerConfig), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("decoding failure: %v", err)
	}

	config, ok := obj.(*kopsproviderconfig.KopsProviderConfig)
	if !ok {
		return nil, fmt.Errorf("failure to cast to KopsProviderConfig; type: %v", gvk)
	}

	return config, nil
}

// The two machines differ in a way that requires an update
func (a *BaremetalActuator) requiresUpdate(l *clusterv1.Machine, r *clusterv1.Machine) bool {
	// Do not want status changes. Do want changes that impact machine provisioning
	return !reflect.DeepEqual(l.Spec.ObjectMeta, r.Spec.ObjectMeta) ||
		!reflect.DeepEqual(l.Spec.ProviderConfig, r.Spec.ProviderConfig) ||
		!reflect.DeepEqual(l.Spec.Roles, r.Spec.Roles) ||
		!reflect.DeepEqual(l.Spec.Versions, r.Spec.Versions) ||
		l.ObjectMeta.Name != r.ObjectMeta.Name ||
		l.ObjectMeta.UID != r.ObjectMeta.UID
}
