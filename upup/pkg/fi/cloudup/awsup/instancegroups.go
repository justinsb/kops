package awsup

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
	"k8s.io/client-go/pkg/api/v1"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/instancegroups"
)

// FindAutoscalingGroups finds autoscaling groups matching the specified tags
// This isn't entirely trivial because autoscaling doesn't let us filter with as much precision as we would like
func FindAutoscalingGroups(cloud AWSCloud, tags map[string]string) ([]*autoscaling.Group, error) {
	var asgs []*autoscaling.Group

	glog.V(2).Infof("Listing all Autoscaling groups matching cluster tags")
	var asgNames []*string
	{
		var asFilters []*autoscaling.Filter
		for _, v := range tags {
			// Not an exact match, but likely the best we can do
			asFilters = append(asFilters, &autoscaling.Filter{
				Name:   aws.String("value"),
				Values: []*string{aws.String(v)},
			})
		}
		request := &autoscaling.DescribeTagsInput{
			Filters: asFilters,
		}

		err := cloud.Autoscaling().DescribeTagsPages(request, func(p *autoscaling.DescribeTagsOutput, lastPage bool) bool {
			for _, t := range p.Tags {
				switch *t.ResourceType {
				case "auto-scaling-group":
					asgNames = append(asgNames, t.ResourceId)
				default:
					glog.Warningf("Unknown resource type: %v", *t.ResourceType)

				}
			}
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("error listing autoscaling cluster tags: %v", err)
		}
	}

	if len(asgNames) != 0 {
		request := &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: asgNames,
		}
		err := cloud.Autoscaling().DescribeAutoScalingGroupsPages(request, func(p *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
			for _, asg := range p.AutoScalingGroups {
				if !MatchesAsgTags(tags, asg.Tags) {
					// We used an inexact filter above
					continue
				}
				// Check for "Delete in progress" (the only use of .Status)
				if asg.Status != nil {
					glog.Warningf("Skipping ASG %v (which matches tags): %v", *asg.AutoScalingGroupARN, *asg.Status)
					continue
				}
				asgs = append(asgs, asg)
			}
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("error listing autoscaling groups: %v", err)
		}

	}

	return asgs, nil
}

func MatchesAsgTags(tags map[string]string, actual []*autoscaling.TagDescription) bool {
	for k, v := range tags {
		found := false
		for _, a := range actual {
			if aws.StringValue(a.Key) == k {
				if aws.StringValue(a.Value) == v {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// FindCloudInstanceGroups joins data from the cloud and the instance groups into a map that can be used for updates.
func (c *awsCloudImplementation) FindCloudInstanceGroups(cluster *api.Cluster, igs []*api.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*instancegroups.CloudInstanceGroup, error) {
	groups := make(map[string]*instancegroups.CloudInstanceGroup)

	tags := c.Tags()

	asgs, err := FindAutoscalingGroups(c, tags)
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[string]*v1.Node)
	for i := range nodes {
		node := &nodes[i]
		awsID := node.Spec.ExternalID
		nodeMap[awsID] = node
	}

	for _, asg := range asgs {
		name := aws.StringValue(asg.AutoScalingGroupName)
		var instancegroup *api.InstanceGroup
		for _, g := range igs {
			var asgName string
			switch g.Spec.Role {
			case api.InstanceGroupRoleMaster:
				asgName = g.ObjectMeta.Name + ".masters." + cluster.ObjectMeta.Name
			case api.InstanceGroupRoleNode:
				asgName = g.ObjectMeta.Name + "." + cluster.ObjectMeta.Name
			case api.InstanceGroupRoleBastion:
				asgName = g.ObjectMeta.Name + "." + cluster.ObjectMeta.Name
			default:
				glog.Warningf("Ignoring InstanceGroup of unknown role %q", g.Spec.Role)
				continue
			}

			if name == asgName {
				if instancegroup != nil {
					return nil, fmt.Errorf("Found multiple instance groups matching ASG %q", asgName)
				}
				instancegroup = g
			}
		}
		if instancegroup == nil {
			if warnUnmatched {
				glog.Warningf("Found ASG with no corresponding instance group %q", name)
			}
			continue
		}
		group := buildCloudInstanceGroup(instancegroup, asg, nodeMap)
		groups[instancegroup.ObjectMeta.Name] = group
	}

	return groups, nil
}

func buildCloudInstanceGroup(ig *api.InstanceGroup, g *autoscaling.Group, nodeMap map[string]*v1.Node) *instancegroups.CloudInstanceGroup {
	n := &instancegroups.CloudInstanceGroup{
		Id:      aws.StringValue(g.AutoScalingGroupName),
		MinSize: int(aws.Int64Value(g.MinSize)),
		MaxSize: int(aws.Int64Value(g.MaxSize)),

		InstanceGroup: ig,
		CloudObject:   g,
	}

	readyLaunchConfigurationName := aws.StringValue(g.LaunchConfigurationName)

	for _, i := range g.Instances {
		c := &instancegroups.CloudInstanceGroupInstance{ID: aws.StringValue(i.InstanceId), CloudObject: i}

		node := nodeMap[aws.StringValue(i.InstanceId)]
		if node != nil {
			c.Node = node
		}

		if readyLaunchConfigurationName == aws.StringValue(i.LaunchConfigurationName) {
			n.Ready = append(n.Ready, c)
		} else {
			n.NeedUpdate = append(n.NeedUpdate, c)
		}
	}

	if len(n.NeedUpdate) == 0 {
		n.Status = "Ready"
	} else {
		n.Status = "NeedsUpdate"
	}

	return n
}

func (c *awsCloudImplementation) DeleteCloudInstanceGroup(g *instancegroups.CloudInstanceGroup) error {
	// TODO: Graceful?

	asg := g.CloudObject.(*autoscaling.Group)

	// Delete ASG
	{
		asgName := aws.StringValue(asg.AutoScalingGroupName)
		glog.V(2).Infof("Deleting autoscaling group %q", asgName)
		request := &autoscaling.DeleteAutoScalingGroupInput{
			AutoScalingGroupName: asg.AutoScalingGroupName,
			ForceDelete:          aws.Bool(true),
		}
		_, err := c.Autoscaling().DeleteAutoScalingGroup(request)
		if err != nil {
			return fmt.Errorf("error deleting autoscaling group %q: %v", asgName, err)
		}
	}

	// Delete LaunchConfig
	{
		lcName := aws.StringValue(asg.LaunchConfigurationName)
		glog.V(2).Infof("Deleting autoscaling launch configuration %q", lcName)
		request := &autoscaling.DeleteLaunchConfigurationInput{
			LaunchConfigurationName: asg.LaunchConfigurationName,
		}
		_, err := c.Autoscaling().DeleteLaunchConfiguration(request)
		if err != nil {
			return fmt.Errorf("error deleting autoscaling launch configuration %q: %v", lcName, err)
		}
	}

	return nil
}

// DeleteAWSInstance deletes an EC2 AWS Instance.
func (c *awsCloudImplementation) DeleteCloudInstanceGroupMember(g *instancegroups.CloudInstanceGroup, u *instancegroups.CloudInstanceGroupInstance) error {
	asg := g.CloudObject.(*autoscaling.Group)
	awsInstance := u.CloudObject.(*autoscaling.Instance)

	instanceId := u.ID
	nodeName := ""
	if u.Node != nil {
		nodeName = u.Node.Name
	}

	if nodeName != "" {
		glog.Infof("Stopping instance %q, node %q, in AWS ASG %q.", instanceId, nodeName, asg.AutoScalingGroupName)
	} else {
		glog.Infof("Stopping instance %q, in AWS ASG %q.", instanceId, asg.AutoScalingGroupName)
	}

	request := &autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     awsInstance.InstanceId,
		ShouldDecrementDesiredCapacity: aws.Bool(false),
	}

	if _, err := c.Autoscaling().TerminateInstanceInAutoScalingGroup(request); err != nil {
		if nodeName != "" {
			return fmt.Errorf("error deleting instance %q, node %q: %v", instanceId, nodeName, err)
		}
		return fmt.Errorf("error deleting instance %q: %v", instanceId, err)
	}

	return nil
}

var _ instancegroups.HasCloudInstanceGroups = &awsCloudImplementation{}
