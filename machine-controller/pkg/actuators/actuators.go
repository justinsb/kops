package actuators

import (
	"k8s.io/kube-deploy/cluster-api-gcp/cloud"
	"k8s.io/kube-deploy/cluster-api/client"
	"k8s.io/kops/machine-controller/pkg/actuators/baremetal"
	"fmt"
	"k8s.io/kops/pkg/client/simple"
)

func NewKopsMachineActuator(cloud string, clientset simple.Clientset, clusterName string, sshUsername string, sshPrivateKeyPath string, machineClient client.MachinesInterface) (cloud.MachineActuator, error) {
	switch cloud {
	case "baremetal":
		return baremetal.NewActuator(clientset, clusterName, sshUsername, sshPrivateKeyPath, machineClient)
	default:
		return nil, fmt.Errorf("unknown cloud provider: %s\n", cloud)
	}
}
