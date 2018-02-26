package actuators

import (
	"fmt"

	"k8s.io/kops/machine-controller/pkg/actuators/baremetal"
	"k8s.io/kops/machine-controller/pkg/actuators/vsphere"
	"k8s.io/kops/machine-controller/pkg/config"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kube-deploy/cluster-api-gcp/cloud"
	"k8s.io/kube-deploy/cluster-api/client"
)

func NewKopsMachineActuator(conf *config.Configuration, clientset simple.Clientset, machineClient client.MachinesInterface) (cloud.MachineActuator, error) {
	cloud := conf.Cloud
	switch cloud {
	case "baremetal":
		return baremetal.NewActuator(conf, clientset, machineClient)
	case "vsphere":
		return vsphere.NewActuator(conf, clientset, machineClient)
	default:
		return nil, fmt.Errorf("unknown cloud provider: %s\n", cloud)
	}
}
