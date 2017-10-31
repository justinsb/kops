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

package awsmodel

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/model/defaults"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awstasks"
	"strings"
)

const (
	DefaultVolumeType = "gp2"
	DefaultVolumeIops = 100
)

// AutoscalingGroupModelBuilder configures AutoscalingGroup objects
type AutoscalingGroupModelBuilder struct {
	*AWSModelContext

	BootstrapScript *model.BootstrapScript
	Lifecycle       *fi.Lifecycle
}

var _ fi.ModelBuilder = &AutoscalingGroupModelBuilder{}

func (b *AutoscalingGroupModelBuilder) Build(c *fi.ModelBuilderContext) error {
	var err error
	for _, ig := range b.InstanceGroups {
		name := b.AutoscalingGroupName(ig)

		volumeSize := fi.Int32Value(ig.Spec.RootVolumeSize)
		if volumeSize == 0 {
			volumeSize, err = defaults.DefaultInstanceGroupVolumeSize(ig.Spec.Role)
			if err != nil {
				return err
			}
		}
		volumeType := fi.StringValue(ig.Spec.RootVolumeType)
		volumeIops := fi.Int32Value(ig.Spec.RootVolumeIops)
		switch volumeType {
		case "io1":
			if volumeIops == 0 {
				volumeIops = DefaultVolumeIops
			}
		default:
			volumeType = DefaultVolumeType
		}
		var rootVolumeIops *int64
		if volumeType == "io1" {
			rootVolumeIops = i64(int64(volumeIops))
		}

		associatePublicIP, err := b.shouldAssociatePublicIP(ig)
		if err != nil {
			return err
		}

		securityGroups := []*awstasks.SecurityGroup{
			b.LinkToSecurityGroup(ig.Spec.Role),
		}

		for _, id := range ig.Spec.AdditionalSecurityGroups {
			sgTask := &awstasks.SecurityGroup{
				Name:   fi.String(id),
				ID:     fi.String(id),
				Shared: fi.Bool(true),
			}
			if err := c.EnsureTask(sgTask); err != nil {
				return err
			}
			securityGroups = append(securityGroups, sgTask)
		}

		userData, err := b.BootstrapScript.ResourceNodeUp(ig, &b.Cluster.Spec)
		if err != nil {
			return err
		}

		sshKey, err := b.LinkToSSHKey()
		if err != nil {
			return err
		}

		subnetSpecs, err := b.GatherSubnets(ig)
		if err != nil {
			return err
		}
		if len(subnetSpecs) == 0 {
			return fmt.Errorf("could not determine any subnets for InstanceGroup %q; subnets was %s", ig.ObjectMeta.Name, ig.Spec.Subnets)
		}
		var subnets []*awstasks.Subnet
		for _, subnet := range subnetSpecs {
			subnets = append(subnets, b.LinkToSubnet(subnet))
		}

		tags, err := b.CloudTagsForInstanceGroup(ig)
		if err != nil {
			return fmt.Errorf("error building cloud tags: %v", err)
		}

		machineController := "autoscaling"

		// TODO: temporary - feature flag this
		if ig.Spec.Role == kops.InstanceGroupRoleNode {
			machineController = "spotfleet"
			if ig.Spec.MaxPrice == nil {
				ig.Spec.MaxPrice = fi.String("0.01")
			}
		}

		switch machineController {
		case "spotfleet":
			{
				maxPrice := fi.StringValue(ig.Spec.MaxPrice)
				if maxPrice == "" {
					return fmt.Errorf("maxPrice must be specified with spotFleet.  Note that this is price per GB of RAM")
				}

				// TODO: Is it OK that maxPrice is now per GB
				// TODO: Is it OK that minSize & maxSize are now in GB, not in machines - sort of cool though!

				role, err := b.ensureSpotFleetIAMRole(c)
				if err != nil {
					return err
				}

				t := &awstasks.SpotFleet{
					Name:      s(name),
					Lifecycle: b.Lifecycle,

					IAMFleetRole: role,

					SecurityGroups:     securityGroups,
					IAMInstanceProfile: b.LinkToIAMInstanceProfile(ig),
					ImageID:            s(ig.Spec.Image),
					InstanceTypes:      strings.Split(ig.Spec.MachineType, ","),

					RootVolumeSize:         i64(int64(volumeSize)),
					RootVolumeType:         s(volumeType),
					RootVolumeOptimization: ig.Spec.RootVolumeOptimization,
					RootVolumeIops:         rootVolumeIops,

					UserData:          userData,
					SSHKey:            sshKey,
					Subnets:           subnets,
					Tags:              tags,
					AssociatePublicIP: &associatePublicIP,

					BidPricePerUnitHour: fi.String(maxPrice),
				}

				if ig.Spec.Tenancy != "" {
					t.Tenancy = s(ig.Spec.Tenancy)
				}

				targetCapacity := int32(1)
				if ig.Spec.MinSize != nil {
					targetCapacity = fi.Int32Value(ig.Spec.MinSize)
				} else if ig.Spec.Role == kops.InstanceGroupRoleNode {
					targetCapacity = 2
				}

				t.TargetCapacity = i64(int64(targetCapacity))

				c.AddTask(t)
			}
		case "autoscaling":
			{
				lc := &awstasks.LaunchConfiguration{
					Name:      s(name),
					Lifecycle: b.Lifecycle,

					SecurityGroups:     securityGroups,
					IAMInstanceProfile: b.LinkToIAMInstanceProfile(ig),
					ImageID:            s(ig.Spec.Image),
					InstanceType:       s(ig.Spec.MachineType),

					RootVolumeSize:         i64(int64(volumeSize)),
					RootVolumeType:         s(volumeType),
					RootVolumeOptimization: ig.Spec.RootVolumeOptimization,
					RootVolumeIops:         rootVolumeIops,

					UserData:          userData,
					SSHKey:            sshKey,
					AssociatePublicIP: &associatePublicIP,
				}

				if ig.Spec.Tenancy != "" {
					lc.Tenancy = s(ig.Spec.Tenancy)
				}

				if fi.StringValue(ig.Spec.MaxPrice) != "" {
					spotPrice := fi.StringValue(ig.Spec.MaxPrice)
					lc.SpotPrice = spotPrice
				}

				asg := &awstasks.AutoscalingGroup{
					Name:      s(name),
					Lifecycle: b.Lifecycle,

					Subnets: subnets,
					Tags:    tags,

					LaunchConfiguration: lc,
				}

				minSize := int32(1)
				maxSize := int32(1)
				if ig.Spec.MinSize != nil {
					minSize = fi.Int32Value(ig.Spec.MinSize)
				} else if ig.Spec.Role == kops.InstanceGroupRoleNode {
					minSize = 2
				}
				if ig.Spec.MaxSize != nil {
					maxSize = *ig.Spec.MaxSize
				} else if ig.Spec.Role == kops.InstanceGroupRoleNode {
					maxSize = 2
				}

				asg.MinSize = i64(int64(minSize))
				asg.MaxSize = i64(int64(maxSize))

				c.AddTask(lc)
				c.AddTask(asg)
			}

		default:
			return fmt.Errorf("unknown MachineController: %q", machineController)
		}
	}

	// TODO: Collect ASGs, LCs, SpotFleets and remove any that aren't still part of the system
	return nil
}

// shouldAssociatePublicIP determines whether instances in the specified instance group should get public IPs
// TODO: not AWS specific (though currently only AWS supports this); move to general helper function
func (b *AutoscalingGroupModelBuilder) shouldAssociatePublicIP(ig *kops.InstanceGroup) (bool, error) {
	// TODO: Wrapper / helper class to analyze clusters
	subnetMap := make(map[string]*kops.ClusterSubnetSpec)
	for i := range b.Cluster.Spec.Subnets {
		subnet := &b.Cluster.Spec.Subnets[i]
		subnetMap[subnet.Name] = subnet
	}

	var subnetType kops.SubnetType
	for _, subnetName := range ig.Spec.Subnets {
		subnet := subnetMap[subnetName]
		if subnet == nil {
			return false, fmt.Errorf("InstanceGroup %q uses subnet %q that does not exist", ig.ObjectMeta.Name, subnetName)
		}
		if subnetType != "" && subnetType != subnet.Type {
			return false, fmt.Errorf("InstanceGroup %q cannot be in subnets of different Type", ig.ObjectMeta.Name)
		}
		subnetType = subnet.Type
	}

	associatePublicIP := true
	switch subnetType {
	case kops.SubnetTypePublic, kops.SubnetTypeUtility:
		associatePublicIP = true
		if ig.Spec.AssociatePublicIP != nil {
			associatePublicIP = *ig.Spec.AssociatePublicIP
		}

	case kops.SubnetTypePrivate:
		associatePublicIP = false
		if ig.Spec.AssociatePublicIP != nil {
			// This isn't meaningful - private subnets can't have public ip
			//associatePublicIP = *ig.Spec.AssociatePublicIP
			if *ig.Spec.AssociatePublicIP {
				glog.Warningf("Ignoring AssociatePublicIP=true for private InstanceGroup %q", ig.ObjectMeta.Name)
			}
		}

	default:
		return false, fmt.Errorf("unknown subnet type %q", subnetType)
	}
	return associatePublicIP, nil
}

// ensureSpotFleetIAMRole adds the tasks for the IAM spot fleet role, used by spot fleet to terminate instances etc
func (b *AutoscalingGroupModelBuilder) ensureSpotFleetIAMRole(c *fi.ModelBuilderContext) (*awstasks.IAMRole, error) {
	name := b.IAMNameForSpotFleet()

	rolePolicyDoc := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Action": "sts:AssumeRole",
				"Principal": {
					"Service": "spotfleet.amazonaws.com"
				},
				"Effect": "Allow",
				"Sid": ""
			}
		]
	}`

	role := &awstasks.IAMRole{
		Name:               s(name),
		RolePolicyDocument: fi.WrapResource(fi.NewStringResource(rolePolicyDoc)),
	}
	c.EnsureTask(role)

	spotFleetRolePolicy := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ec2:DescribeImages",
                "ec2:DescribeSubnets",
                "ec2:RequestSpotInstances",
                "ec2:TerminateInstances",
                "ec2:DescribeInstanceStatus",
                "ec2:CreateTags"
            ],
            "Resource": [
                "*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": "iam:PassRole",
            "Condition": {
                "StringEquals": {
                    "iam:PassedToService": "ec2.amazonaws.com"
                }
            },
            "Resource": [
                "*"
            ]
        }
    ]
}`

	rolePolicy := &awstasks.IAMRolePolicy{
		Name:           s(name),
		Role:           role,
		PolicyDocument: fi.WrapResource(fi.NewStringResource(spotFleetRolePolicy)),
	}
	c.EnsureTask(rolePolicy)

	// TODO: Ensure this is removed by kops delete cluster

	return role, nil
}
