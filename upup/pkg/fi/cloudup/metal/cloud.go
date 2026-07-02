/*
Copyright 2024 The Kubernetes Authors.

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

package metal

import (
	"fmt"
	"net"

	"github.com/blang/semver/v4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/cloudinstances"
	"k8s.io/kops/upup/pkg/fi"
)

var _ fi.Cloud = (*Cloud)(nil)

// Cloud holds the fi.Cloud implementation for metal resources.
type Cloud struct {
}

// NewCloud returns a Cloud for metal resources.
func NewCloud() (*Cloud, error) {
	cloud := &Cloud{}
	return cloud, nil
}

func (c *Cloud) ProviderID() kops.CloudProviderID {
	return kops.CloudProviderMetal
}
func (c *Cloud) DNS() (dnsprovider.Interface, error) {
	return nil, fmt.Errorf("method metal.Cloud::DNS not implemented")
}

// FindVPCInfo looks up the specified VPC by id, returning info if found, otherwise (nil, nil).
func (c *Cloud) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	return nil, fmt.Errorf("method metal.Cloud::FindVPCInfo not implemented")
}

// DeleteInstance deletes a cloud instance.
func (c *Cloud) DeleteInstance(instance *cloudinstances.CloudInstance) error {
	return fmt.Errorf("method metal.Cloud::DeleteInstance not implemented")
}

// DeregisterInstance drains a cloud instance and loadbalancers.
// For metal there are no cloud-managed load balancers, so this is a no-op.
func (c *Cloud) DeregisterInstance(instance *cloudinstances.CloudInstance) error {
	return nil
}

// DeleteGroup deletes the cloud resources that make up a CloudInstanceGroup, including the instances.
func (c *Cloud) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
	return fmt.Errorf("method metal.Cloud::DeleteGroup not implemented")
}

// DetachInstance causes a cloud instance to no longer be counted against the group's size limits.
func (c *Cloud) DetachInstance(instance *cloudinstances.CloudInstance) error {
	return fmt.Errorf("method metal.Cloud::DetachInstance not implemented")
}

// GetCloudGroups returns a map of cloud instances that back a kops cluster.
// Detached instances must be returned in the NeedUpdate slice.
func (c *Cloud) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	groups := make(map[string]*cloudinstances.CloudInstanceGroup)
	for _, ig := range instancegroups {
		cloudInstanceGroup := &cloudinstances.CloudInstanceGroup{
			HumanName:     ig.ObjectMeta.Name,
			InstanceGroup: ig,
		}
		groups[ig.ObjectMeta.Name] = cloudInstanceGroup
		for i := range nodes {
			node := &nodes[i]

			isControlPlaneNode := false
			for k := range node.Labels {
				if k == "node-role.kubernetes.io/control-plane" {
					isControlPlaneNode = true
				}

			}

			match := true
			switch {
			case ig.Spec.Role.HasControlPlane():
				if !isControlPlaneNode {
					match = false
				}

			case ig.Spec.Role.HasNode():
				if isControlPlaneNode {
					match = false
				}
			}

			// If the node is labeled with its instance group, require an exact match;
			// this avoids matching the same node to multiple instance groups with the same role.
			if igName, found := node.Labels["kops.k8s.io/instancegroup"]; found && igName != ig.ObjectMeta.Name {
				match = false
			}

			if !match {
				continue
			}

			status := cloudinstances.CloudInstanceStatusUpToDate
			if !nodeMatchesKubernetesVersion(node, cluster.Spec.KubernetesVersion) {
				status = cloudinstances.CloudInstanceStatusNeedsUpdate
			}
			if _, err := cloudInstanceGroup.NewCloudInstance(node.Name, status, node); err != nil {
				return nil, fmt.Errorf("building cloud instance for node %q: %w", node.Name, err)
			}
		}

		// We cannot create machines, so the target size is simply the machines that exist.
		numInstances := len(cloudInstanceGroup.Ready) + len(cloudInstanceGroup.NeedUpdate)
		cloudInstanceGroup.MinSize = numInstances
		cloudInstanceGroup.TargetSize = numInstances
		cloudInstanceGroup.MaxSize = numInstances
	}

	return groups, nil
}

// nodeMatchesKubernetesVersion checks whether the kubelet on the node is running the
// kubernetes version specified in the cluster spec.  We cannot (easily) compare the full
// nodeup configuration for bare-metal machines, so we use the kubelet version as the
// signal that the machine needs an in-place update.  Machines can additionally be marked
// for update with the kops.k8s.io/needs-update annotation, or updated with --force.
func nodeMatchesKubernetesVersion(node *v1.Node, kubernetesVersion string) bool {
	kubeletVersion, err := semver.ParseTolerant(node.Status.NodeInfo.KubeletVersion)
	if err != nil {
		klog.Warningf("cannot parse kubelet version %q for node %q; assuming up-to-date", node.Status.NodeInfo.KubeletVersion, node.Name)
		return true
	}
	wantVersion, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		klog.Warningf("cannot parse cluster kubernetes version %q; assuming node %q is up-to-date", kubernetesVersion, node.Name)
		return true
	}
	return kubeletVersion.Major == wantVersion.Major &&
		kubeletVersion.Minor == wantVersion.Minor &&
		kubeletVersion.Patch == wantVersion.Patch
}

// Region returns the cloud region bound to the cloud instance.
// If the region concept does not apply, returns "".
func (c *Cloud) Region() string {
	return ""
}

// FindClusterStatus discovers the status of the cluster, by inspecting the cloud objects
func (c *Cloud) FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error) {
	// etcdStatus, err := findEtcdStatus(c, cluster)
	// if err != nil {
	//      return nil, err
	// }
	klog.Warningf("method metal.Cloud::FindClusterStatus stub-implemented")
	return &kops.ClusterStatus{
		// EtcdClusters: etcdStatus,
	}, nil
}

func (c *Cloud) GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
	var ret []fi.ApiIngressStatus
	publicName := cluster.Spec.API.PublicName
	if publicName == "" {
		return ret, fmt.Errorf("%s must be set for bare metal", kops.HumanPathForClusterField("spec.api.publicName"))
	}
	ip := net.ParseIP(publicName)
	if ip == nil {
		ret = append(ret, fi.ApiIngressStatus{
			Hostname: publicName,
		})
	} else {
		ret = append(ret, fi.ApiIngressStatus{
			IP: publicName,
		})
	}
	return ret, nil
}
