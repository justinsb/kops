/*
Copyright 2016 The Kubernetes Authors.

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

package protokube

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	metav1 "k8s.io/kubernetes/pkg/apis/meta/v1"
	"net"
	"strings"
)

// Keep in sync with dns-controller
const AnnotationNameExternalIP = "dns.alpha.kubernetes.io/external-ip"

// PopulateExternalIP sets the external IP on the node which is our node
func PopulateExternalIP(kubeContext *KubernetesContext, nodeName string, ips []net.IP) error {
	client, err := kubeContext.KubernetesClient()
	if err != nil {
		return err
	}

	glog.V(2).Infof("Querying k8s for node %q", nodeName)
	node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error querying for node %q: %v", nodeName, err)
	}

	if node == nil {
		return fmt.Errorf("could not find node %q", nodeName)
	}

	var annotationString string
	{
		var ipStrings []string
		for _, ip := range ips {
			ipStrings = append(ipStrings, ip.String())
		}
		annotationString = strings.Join(ipStrings, ",")
	}

	actual := node.Annotations[AnnotationNameExternalIP]
	if actual == annotationString {
		glog.V(2).Infof("Node already has correct annotation %s=%s", AnnotationNameExternalIP, actual)
	}

	{
		nodePatchMetadata := &nodePatchMetadata{
			Annotations: map[string]string{AnnotationNameExternalIP: annotationString},
		}
		nodePatch := &nodePatch{
			Metadata: nodePatchMetadata,
		}
		nodePatchJson, err := json.Marshal(nodePatch)
		if err != nil {
			return fmt.Errorf("error building node patch: %v", err)
		}

		glog.V(2).Infof("sending patch for node %q: %q", nodeName, string(nodePatchJson))

		_, err = client.Nodes().Patch(nodeName, api.StrategicMergePatchType, nodePatchJson)
		if err != nil {
			// TODO: Should we keep going?
			return fmt.Errorf("error applying patch to node: %v", err)
		}
	}

	glog.Infof("Patched node with annotation %s=%s", AnnotationNameExternalIP, annotationString)
	return nil
}
