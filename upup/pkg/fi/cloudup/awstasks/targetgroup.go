/*
Copyright 2020 The Kubernetes Authors.

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

package awstasks

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/truncate"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraformWriter"
)

const (
	// TargetGroupAttributeDeregistrationDelayConnectionTerminationEnabled indicates whether
	//the load balancer terminates connections at the end of the deregistration timeout.
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-target-groups.html#deregistration-delay
	TargetGroupAttributeDeregistrationDelayConnectionTerminationEnabled = "deregistration_delay.connection_termination.enabled"
	// TargetGroupAttributeDeregistrationDelayTimeoutSeconds is the amount of time for Elastic Load Balancing
	// to wait before changing the state of a deregistering target from draining to unused.
	// https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-target-groups.html#deregistration-delay
	TargetGroupAttributeDeregistrationDelayTimeoutSeconds = "deregistration_delay.timeout_seconds"
)

// +kops:fitask
type TargetGroup struct {
	Name      *string
	Lifecycle fi.Lifecycle
	VPC       *VPC
	Tags      map[string]string
	Port      *int64
	Protocol  *string

	// networkLoadBalancer, if set, will create a new Target Group for each revision of the Network Load Balancer
	networkLoadBalancer *NetworkLoadBalancer

	// ARN is the Amazon Resource Name for the Target Group
	ARN *string

	// Shared is set if this is an external Target Group (one we don't create or own)
	Shared *bool

	Attributes map[string]string

	Interval           *int64
	HealthyThreshold   *int64
	UnhealthyThreshold *int64

	info     *awsup.TargetGroupInfo
	revision string

	// deletions is a list of previous versions of this object, that we should delete when asked to clean up.
	deletions []fi.CloudupDeletion
}

func (e *TargetGroup) AddRevisionsForNLB(nlb *NetworkLoadBalancer) {
	e.networkLoadBalancer = nlb
}

var _ fi.CloudupHasDependencies = &TargetGroup{}

// GetDependencies returns the dependencies of the TargetGroup task
func (e *TargetGroup) GetDependencies(tasks map[string]fi.CloudupTask) []fi.CloudupTask {
	var deps []fi.CloudupTask
	deps = append(deps, e.VPC)
	deps = append(deps, e.networkLoadBalancer)
	return deps
}

var _ fi.CompareWithID = &TargetGroup{}

func (e *TargetGroup) CompareWithID() *string {
	if e.ARN != nil {
		return e.ARN
	}
	return e.Name
}

func (e *TargetGroup) findLatestTargetGroup(ctx context.Context, cloud awsup.AWSCloud) (*awsup.TargetGroupInfo, error) {
	name := fi.ValueOf(e.Name)

	targetGroups, err := awsup.ListELBV2TargetGroups(ctx, cloud)
	if err != nil {
		return nil, err
	}

	var latest *awsup.TargetGroupInfo
	var latestRevision int
	for _, targetGroup := range targetGroups {
		if aws.StringValue(targetGroup.TargetGroup.TargetGroupName) != name && targetGroup.NameTag() != name {
			continue
		}
		revisionTag, _ := targetGroup.GetTag(KopsResourceRevisionTag)

		revision := -1
		if revisionTag == "" {
			revision = 0
		} else {
			n, err := strconv.Atoi(revisionTag)
			if err != nil {
				klog.Warningf("ignoring target group %q with revision %q", targetGroup.ARN(), revision)
				continue
			}
			revision = n
		}

		if latest == nil || revision > latestRevision {
			latestRevision = revision
			latest = targetGroup
		}
	}

	if latest != nil && e.networkLoadBalancer != nil {
		matchRevision := e.networkLoadBalancer.revision
		arn := e.networkLoadBalancer.loadBalancerArn
		if arn == "" {
			return nil, fmt.Errorf("load balancer not ready (no ARN)")
		}
		revisionTag, _ := latest.GetTag(KopsResourceRevisionTag)

		// klog.Infof("nlb is %+v", e.networkLoadBalancer)
		if revisionTag != matchRevision {
			klog.Warningf("found target group but revision %q does not match load balancer revision %q; will create a new target group", revisionTag, matchRevision)
			latest = nil
		}
	}

	// Stash deletions for later
	for _, targetGroup := range targetGroups {
		if aws.StringValue(targetGroup.TargetGroup.TargetGroupName) != name && targetGroup.NameTag() != name {
			continue
		}
		if latest != nil && latest.ARN() == targetGroup.ARN() {
			continue
		}

		e.deletions = append(e.deletions, buildDeleteTargetGroup(targetGroup))
	}

	return latest, nil
}

func (e *TargetGroup) findTargetGroupByARN(ctx context.Context, cloud awsup.AWSCloud) (*awsup.TargetGroupInfo, error) {
	request := &elbv2.DescribeTargetGroupsInput{}
	request.TargetGroupArns = []*string{e.ARN}

	response, err := cloud.ELBV2().DescribeTargetGroups(request)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == elbv2.ErrCodeTargetGroupNotFoundException {
			if !fi.ValueOf(e.Shared) {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("error describing targetgroup %s: %v", *e.Name, err)
	}

	if len(response.TargetGroups) > 1 {
		return nil, fmt.Errorf("found %d TargetGroups with ID %q, expected 1", len(response.TargetGroups), fi.ValueOf(e.Name))
	} else if len(response.TargetGroups) == 0 {
		return nil, nil
	}

	tg := response.TargetGroups[0]

	tagResponse, err := cloud.ELBV2().DescribeTagsWithContext(ctx, &elbv2.DescribeTagsInput{
		ResourceArns: []*string{tg.TargetGroupArn},
	})
	if err != nil {
		return nil, err
	}

	info := &awsup.TargetGroupInfo{
		TargetGroup: tg,
	}

	for _, t := range tagResponse.TagDescriptions {
		info.Tags = append(info.Tags, t.Tags...)
	}

	return info, nil
}

func (e *TargetGroup) Find(c *fi.CloudupContext) (*TargetGroup, error) {
	ctx := c.Context()
	cloud := c.T.Cloud.(awsup.AWSCloud)

	var targetGroupInfo *awsup.TargetGroupInfo

	if e.ARN == nil {
		var err error
		targetGroupInfo, err = e.findLatestTargetGroup(ctx, cloud)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		targetGroupInfo, err = e.findTargetGroupByARN(ctx, cloud)
		if err != nil {
			return nil, err
		}
	}

	if targetGroupInfo == nil {
		return nil, nil
	}

	tg := targetGroupInfo.TargetGroup
	actual := &TargetGroup{
		Name:               tg.TargetGroupName,
		Port:               tg.Port,
		Protocol:           tg.Protocol,
		ARN:                tg.TargetGroupArn,
		Interval:           tg.HealthCheckIntervalSeconds,
		HealthyThreshold:   tg.HealthyThresholdCount,
		UnhealthyThreshold: tg.UnhealthyThresholdCount,
		VPC:                &VPC{ID: tg.VpcId},
	}
	actual.info = targetGroupInfo
	e.info = targetGroupInfo
	actual.revision, _ = targetGroupInfo.GetTag(KopsResourceRevisionTag)
	e.revision = actual.revision

	// Interval cannot be changed after TargetGroup creation
	e.Interval = actual.Interval

	e.ARN = tg.TargetGroupArn
	tags := make(map[string]string)
	for _, tag := range targetGroupInfo.Tags {
		k := fi.ValueOf(tag.Key)
		v := fi.ValueOf(tag.Value)
		if k == KopsResourceRevisionTag {
			actual.revision = v
			continue
		}
		tags[k] = v
	}
	actual.Tags = tags

	attrResp, err := cloud.ELBV2().DescribeTargetGroupAttributes(&elbv2.DescribeTargetGroupAttributesInput{
		TargetGroupArn: tg.TargetGroupArn,
	})
	if err != nil {
		return nil, err
	}
	attributes := make(map[string]string)
	for _, attr := range attrResp.Attributes {
		if _, ok := e.Attributes[fi.ValueOf(attr.Key)]; ok {
			attributes[fi.ValueOf(attr.Key)] = fi.ValueOf(attr.Value)
		}
	}
	if len(attributes) > 0 {
		actual.Attributes = attributes
	}

	// Prevent spurious changes
	actual.Lifecycle = e.Lifecycle
	actual.Shared = e.Shared

	if e.Name != nil {
		actual.Name = e.Name
	}

	return actual, nil
}

func (e *TargetGroup) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(e, c)
}

func (_ *TargetGroup) ShouldCreate(a, e, changes *TargetGroup) (bool, error) {
	if fi.ValueOf(e.Shared) {
		return false, nil
	}
	return true, nil
}

func (s *TargetGroup) CheckChanges(a, e, changes *TargetGroup) error {
	return nil
}

func (_ *TargetGroup) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *TargetGroup) error {
	shared := fi.ValueOf(e.Shared)
	if shared {
		return nil
	}

	tags := make(map[string]string)
	for k, v := range e.Tags {
		tags[k] = v
	}
	if a != nil {
		if a.revision != "" {
			tags[KopsResourceRevisionTag] = a.revision
		}
	}

	if e.networkLoadBalancer != nil {
		if e.networkLoadBalancer.loadBalancerArn == "" {
			return fmt.Errorf("load balancer not yet ready (arn is empty)")
		}
		nlbRevision := e.networkLoadBalancer.revision
		if nlbRevision != "" {
			tags[KopsResourceRevisionTag] = nlbRevision
		}
	}

	// You register targets for your Network Load Balancer with a target group. By default, the load balancer sends requests
	// to registered targets using the port and protocol that you specified for the target group. You can override this port
	// when you register each target with the target group.

	if a == nil {
		createTargetGroupName := *e.Name
		if tags[KopsResourceRevisionTag] != "" {
			s := *e.Name + tags[KopsResourceRevisionTag]
			// We always compute the hash and add it, lest we trick users into assuming that we never do this
			opt := truncate.TruncateStringOptions{
				MaxLength:     32,
				AlwaysAddHash: true,
				HashLength:    6,
			}
			createTargetGroupName = truncate.TruncateString(s, opt)
		}

		request := &elbv2.CreateTargetGroupInput{
			Name:                       &createTargetGroupName,
			Port:                       e.Port,
			Protocol:                   e.Protocol,
			VpcId:                      e.VPC.ID,
			HealthCheckIntervalSeconds: e.Interval,
			HealthyThresholdCount:      e.HealthyThreshold,
			UnhealthyThresholdCount:    e.UnhealthyThreshold,
			Tags:                       awsup.ELBv2Tags(tags),
		}

		klog.V(2).Infof("Creating Target Group for NLB")
		response, err := t.Cloud.ELBV2().CreateTargetGroup(request)
		if err != nil {
			return fmt.Errorf("creating NLB target group: %w", err)
		}

		if err := ModifyTargetGroupAttributes(t.Cloud, response.TargetGroups[0].TargetGroupArn, e.Attributes); err != nil {
			return err
		}

		// Avoid spurious changes
		e.ARN = response.TargetGroups[0].TargetGroupArn

		// TODO: Set revision or info?
	} else {
		if a.ARN != nil {
			if err := t.AddELBV2Tags(fi.ValueOf(a.ARN), e.Tags); err != nil {
				return err
			}
			if err := ModifyTargetGroupAttributes(t.Cloud, a.ARN, e.Attributes); err != nil {
				return err
			}
		}
	}
	return nil
}

func ModifyTargetGroupAttributes(cloud awsup.AWSCloud, arn *string, attributes map[string]string) error {
	klog.V(2).Infof("Modifying Target Group attributes for NLB")
	attrReq := &elbv2.ModifyTargetGroupAttributesInput{
		Attributes:     []*elbv2.TargetGroupAttribute{},
		TargetGroupArn: arn,
	}
	for k, v := range attributes {
		attrReq.Attributes = append(attrReq.Attributes, &elbv2.TargetGroupAttribute{
			Key:   fi.PtrTo(k),
			Value: fi.PtrTo(v),
		})
	}
	if _, err := cloud.ELBV2().ModifyTargetGroupAttributes(attrReq); err != nil {
		return fmt.Errorf("error modifying target group attributes for NLB : %v", err)
	}
	return nil
}

// OrderTargetGroupsByName implements sort.Interface for []OrderTargetGroupsByName, based on port number
type OrderTargetGroupsByName []*TargetGroup

func (a OrderTargetGroupsByName) Len() int      { return len(a) }
func (a OrderTargetGroupsByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a OrderTargetGroupsByName) Less(i, j int) bool {
	return fi.ValueOf(a[i].Name) < fi.ValueOf(a[j].Name)
}

type terraformTargetGroup struct {
	Name                  string                          `cty:"name"`
	Port                  int64                           `cty:"port"`
	Protocol              string                          `cty:"protocol"`
	VPCID                 *terraformWriter.Literal        `cty:"vpc_id"`
	ConnectionTermination string                          `cty:"connection_termination"`
	DeregistrationDelay   string                          `cty:"deregistration_delay"`
	Tags                  map[string]string               `cty:"tags"`
	HealthCheck           terraformTargetGroupHealthCheck `cty:"health_check"`
}

type terraformTargetGroupHealthCheck struct {
	Interval           int64  `cty:"interval"`
	HealthyThreshold   int64  `cty:"healthy_threshold"`
	UnhealthyThreshold int64  `cty:"unhealthy_threshold"`
	Protocol           string `cty:"protocol"`
}

func (_ *TargetGroup) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *TargetGroup) error {
	shared := fi.ValueOf(e.Shared)
	if shared {
		return nil
	}

	if e.VPC == nil {
		return fmt.Errorf("Missing VPC task from target group:\n%v\n%v", e, e.VPC)
	}

	tf := &terraformTargetGroup{
		Name:     *e.Name,
		Port:     *e.Port,
		Protocol: *e.Protocol,
		VPCID:    e.VPC.TerraformLink(),
		Tags:     e.Tags,
		HealthCheck: terraformTargetGroupHealthCheck{
			Interval:           *e.Interval,
			HealthyThreshold:   *e.HealthyThreshold,
			UnhealthyThreshold: *e.UnhealthyThreshold,
			Protocol:           elbv2.ProtocolEnumTcp,
		},
	}

	for attr, val := range e.Attributes {
		if attr == TargetGroupAttributeDeregistrationDelayConnectionTerminationEnabled {
			tf.ConnectionTermination = val
		}
		if attr == TargetGroupAttributeDeregistrationDelayTimeoutSeconds {
			tf.DeregistrationDelay = val
		}
	}

	return t.RenderResource("aws_lb_target_group", *e.Name, tf)
}

func (e *TargetGroup) TerraformLink() *terraformWriter.Literal {
	shared := fi.ValueOf(e.Shared)
	if shared {
		if e.ARN != nil {
			return terraformWriter.LiteralFromStringValue(*e.ARN)
		} else {
			klog.Warningf("ID not set on shared Target Group %v", e)
		}
	}
	return terraformWriter.LiteralProperty("aws_lb_target_group", *e.Name, "id")
}

var _ fi.CloudupProducesDeletions = &TargetGroup{}

// FindDeletions is responsible for finding launch templates which can be deleted
func (e *TargetGroup) FindDeletions(c *fi.CloudupContext) ([]fi.CloudupDeletion, error) {
	var removals []fi.CloudupDeletion
	for _, d := range e.deletions {
		removals = append(removals, d)
	}
	// removals = append(removals, e.deletions...)

	return removals, nil
}

// deleteTargetGroup tracks a TargetGroup that we're going to delete
// It implements fi.CloudupDeletion
type deleteTargetGroup struct {
	fi.CloudupDeletionBase
	obj *awsup.TargetGroupInfo
}

func buildDeleteTargetGroup(obj *awsup.TargetGroupInfo) *deleteTargetGroup {
	d := &deleteTargetGroup{}
	d.obj = obj
	d.Info.Type = "target-group"
	d.Info.ID = obj.ARN()
	d.Info.Name = obj.NameTag()
	d.Info.DeferDeletion = true
	return d
}

var _ fi.CloudupDeletion = &deleteTargetGroup{}

func (d *deleteTargetGroup) Delete(ctx context.Context, t fi.CloudupTarget) error {
	awsTarget, ok := t.(*awsup.AWSAPITarget)
	if !ok {
		return fmt.Errorf("unexpected target type for deletion: %T", t)
	}

	arn := d.obj.ARN()
	klog.V(2).Infof("deleting target group %q", arn)
	if _, err := awsTarget.Cloud.ELBV2().DeleteTargetGroupWithContext(ctx, &elbv2.DeleteTargetGroupInput{
		TargetGroupArn: &arn,
	}); err != nil {
		return fmt.Errorf("error deleting ELB TargetGroup %q: %w", arn, err)
	}

	return nil
}
