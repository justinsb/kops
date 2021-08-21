/*
Copyright 2019 The Kubernetes Authors.

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

package gcemodel

import (
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/cidr"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gcetasks"
)

// ExternalAccessModelBuilder configures security group rules for external access
// (SSHAccess, KubernetesAPIAccess)
type ExternalAccessModelBuilder struct {
	*GCEModelContext
	Lifecycle fi.Lifecycle
}

var _ fi.ModelBuilder = &ExternalAccessModelBuilder{}

func (b *ExternalAccessModelBuilder) Build(c *fi.ModelBuilderContext) error {
	klog.Warningf("TODO: Harmonize gcemodel ExternalAccessModelBuilder with awsmodel")
	if len(b.Cluster.Spec.KubernetesAPIAccess) == 0 {
		klog.Warningf("KubernetesAPIAccess is empty")
	}

	if len(b.Cluster.Spec.SSHAccess) == 0 {
		klog.Warningf("SSHAccess is empty")
	}

	// SSH is open to AdminCIDR set
	if b.UsesSSHBastion() {
		// If we are using a bastion, we only access through the bastion
		// This is admittedly a little odd... adding a bastion shuts down direct access to the masters/nodes
		// But I think we can always add more permissions in this case later, but we can't easily take them away
		klog.V(2).Infof("bastion is in use; won't configure SSH access to master / node instances")
	} else {
		sshAccess, err := cidr.NewSet(b.Cluster.Spec.SSHAccess)
		if err != nil {
			return err
		}

		AddFirewallRulesTasks(c, &gcetasks.FirewallRule{
			Name:       s(b.SafeObjectName("ssh-external-to-master")),
			Lifecycle:  b.Lifecycle,
			TargetTags: []string{b.GCETagForRole(kops.InstanceGroupRoleMaster)},
			Allowed:    []string{"tcp:22"},
			Network:    b.LinkToNetwork(),
		})

		AddFirewallRulesTasks(c, &gcetasks.FirewallRule{
			Name:         s(b.SafeObjectName("ssh-external-to-node")),
			Lifecycle:    b.Lifecycle,
			TargetTags:   []string{b.GCETagForRole(kops.InstanceGroupRoleNode)},
			Allowed:      []string{"tcp:22"},
			SourceRanges: sshAccess.ToStrings(),
			Network:      b.LinkToNetwork(),
		})
	}

	// NodePort access
	{
		nodePortRange, err := b.NodePortRange()
		if err != nil {
			return err
		}
		nodePortAccess, err := cidr.NewSet(b.Cluster.Spec.NodePortAccess)
		if err != nil {
			return err
		}

		nodePortRangeString := nodePortRange.String()
		AddFirewallRulesTasks(c, &gcetasks.FirewallRule{
			Name:       s(b.SafeObjectName("nodeport-external-to-node")),
			Lifecycle:  b.Lifecycle,
			TargetTags: []string{b.GCETagForRole(kops.InstanceGroupRoleNode)},
			Allowed: []string{
				"tcp:" + nodePortRangeString,
				"udp:" + nodePortRangeString,
			},
			SourceRanges: nodePortAccess.ToStrings(),
			Network:      b.LinkToNetwork(),
		})
	}

	if !b.UseLoadBalancerForAPI() {
		// Configuration for the master, when not using a Loadbalancer (ELB)
		// We expect that either the IP address is published, or DNS is set up to point to the IPs
		// We need to open security groups directly to the master nodes (instead of via the ELB)

		kubernetesAPIAccess, err := cidr.NewSet(b.Cluster.Spec.KubernetesAPIAccess)
		if err != nil {
			return err
		}

		// HTTPS to the master is allowed (for API access)
		AddFirewallRulesTasks(c, &gcetasks.FirewallRule{
			Name:         s(b.SafeObjectName("kubernetes-master-https")),
			Lifecycle:    b.Lifecycle,
			TargetTags:   []string{b.GCETagForRole(kops.InstanceGroupRoleMaster)},
			Allowed:      []string{"tcp:443"},
			SourceRanges: kubernetesAPIAccess.ToStrings(),
			Network:      b.LinkToNetwork(),
		})
	}

	return nil
}
