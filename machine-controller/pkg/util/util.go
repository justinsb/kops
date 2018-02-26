/*
Copyright 2017 The Kubernetes Authors.

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

package util

import (
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "k8s.io/kube-deploy/cluster-api/api/cluster/v1alpha1"
	"k8s.io/kube-deploy/cluster-api/client"
)

//const (
//	CharSet = "0123456789abcdefghijklmnopqrstuvwxyz"
//)
//
//var (
//	r = rand.New(rand.NewSource(time.Now().UnixNano()))
//)
//
//func RandomToken() string {
//	return fmt.Sprintf("%s.%s", RandomString(6), RandomString(16))
//}
//
//func RandomString(n int) string {
//	result := make([]byte, n)
//	for i := range result {
//		result[i] = CharSet[r.Intn(len(CharSet))]
//	}
//	return string(result)
//}
//
//func GetMaster(machines []*clusterv1.Machine) *clusterv1.Machine {
//	for _, machine := range machines {
//		if apiutil.IsMaster(machine) {
//			return machine
//		}
//	}
//	return nil
//}
//
//func MachineP(machines []clusterv1.Machine) []*clusterv1.Machine {
//	// Convert to list of pointers
//	var ret []*clusterv1.Machine
//	for _, machine := range machines {
//		ret = append(ret, machine.DeepCopy())
//	}
//	return ret
//}
//
//func NewClientSet(configPath string) (*apiextensionsclient.Clientset, error) {
//	config, err := clientcmd.BuildConfigFromFlags("", configPath)
//	if err != nil {
//		return nil, err
//	}
//
//	cs, err := apiextensionsclient.NewForConfig(config)
//	if err != nil {
//		return nil, err
//	}
//
//	return cs, nil
//}

func GetCurrentMachineIfExists(machineClient client.MachinesInterface, machine *clusterv1.Machine) (*clusterv1.Machine, error) {
	return GetMachineIfExists(machineClient, machine.ObjectMeta.Name, machine.ObjectMeta.UID)
}

func GetMachineIfExists(machineClient client.MachinesInterface, name string, uid types.UID) (*clusterv1.Machine, error) {
	if machineClient == nil {
		// Being called before k8s is setup as part of master VM creation
		glog.Infof("machineClient was nil; assuming bootstrapping")
		return nil, nil
	}

	// Machines are identified by name and UID
	machine, err := machineClient.Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			glog.Infof("could not find machine %q: %v", name, err)
			return nil, nil
		}
		glog.Infof("error querying for machine %q: %v", name, err)
		return nil, err
	}

	if machine.ObjectMeta.UID != uid {
		glog.Infof("found machine but UID did not match: %q vs %q", machine.ObjectMeta.UID, uid)
		return nil, nil
	}
	return machine, nil
}
