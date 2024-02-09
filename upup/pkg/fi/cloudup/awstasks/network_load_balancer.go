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

package awstasks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/route53"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/truncate"
	"k8s.io/kops/pkg/wellknownservices"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraformWriter"
)

// KopsResourceRevisionTag is the tag used to store the revision timestamp,
const KopsResourceRevisionTag = awsup.KopsResourceRevisionTag

// NetworkLoadBalancer manages an NLB.  We find the existing NLB using the Name tag.
var _ DNSTarget = &NetworkLoadBalancer{}

// +kops:fitask
type NetworkLoadBalancer struct {
	// We use the Name tag to find the existing NLB, because we are (more or less) unrestricted when
	// it comes to tag values, but the LoadBalancerName is length limited
	Name      *string
	Lifecycle fi.Lifecycle

	// LoadBalancerBaseName is the base name to use when naming load balancers in NLB.
	// The full, stable name will be in the Name tag.
	// (NLB is restricted as to names, so we have limited choices!)
	LoadBalancerBaseName *string

	CLBName *string

	DNSName      *string
	HostedZoneId *string

	SubnetMappings []*SubnetMapping
	SecurityGroups []*SecurityGroup

	Scheme *string

	CrossZoneLoadBalancing *bool

	IpAddressType *string

	Tags map[string]string

	Type *string

	VPC       *VPC
	AccessLog *NetworkLoadBalancerAccessLog

	// WellKnownServices indicates which services are supported by this resource.
	// This field is internal and is not rendered to the cloud.
	WellKnownServices []wellknownservices.WellKnownService

	// waitForLoadBalancerReady controls whether we wait for the load balancer to be ready before completing the "Render" operation.
	waitForLoadBalancerReady bool

	// After this is found/created, we store the ARN
	loadBalancerArn string

	// After this is found/created, we store the revision
	revision string

	// deletions is a list of previous versions of this object, that we should delete when asked to clean up.
	deletions []fi.CloudupDeletion
}

func (e *NetworkLoadBalancer) SetWaitForLoadBalancerReady(v bool) {
	e.waitForLoadBalancerReady = v
}

var _ fi.CompareWithID = &NetworkLoadBalancer{}
var _ fi.CloudupTaskNormalize = &NetworkLoadBalancer{}
var _ fi.CloudupProducesDeletions = &NetworkLoadBalancer{}

func (e *NetworkLoadBalancer) CompareWithID() *string {
	return e.Name
}

// The load balancer name 'api.renamenlbcluster.k8s.local' can only contain characters that are alphanumeric characters and hyphens(-)\n\tstatus code: 400,
func findNetworkLoadBalancerByLoadBalancerName(cloud awsup.AWSCloud, loadBalancerName string) (*elbv2.LoadBalancer, error) {
	request := &elbv2.DescribeLoadBalancersInput{
		Names: []*string{&loadBalancerName},
	}
	found, err := describeNetworkLoadBalancers(cloud, request, func(lb *elbv2.LoadBalancer) bool {
		// TODO: Filter by cluster?

		if aws.StringValue(lb.LoadBalancerName) == loadBalancerName {
			return true
		}

		klog.Warningf("Got NLB with unexpected name: %q", aws.StringValue(lb.LoadBalancerName))
		return false
	})
	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			if awsError.Code() == "LoadBalancerNotFound" {
				return nil, nil
			}
		}

		return nil, fmt.Errorf("error listing NLBs: %v", err)
	}

	if len(found) == 0 {
		return nil, nil
	}

	if len(found) != 1 {
		return nil, fmt.Errorf("Found multiple NLBs with name %q", loadBalancerName)
	}

	return found[0], nil
}

func findNetworkLoadBalancerByAlias(cloud awsup.AWSCloud, alias *route53.AliasTarget) (*elbv2.LoadBalancer, error) {
	// TODO: Any way to avoid listing all NLBs?
	request := &elbv2.DescribeLoadBalancersInput{}

	dnsName := aws.StringValue(alias.DNSName)
	matchDnsName := strings.TrimSuffix(dnsName, ".")
	if matchDnsName == "" {
		return nil, fmt.Errorf("DNSName not set on AliasTarget")
	}

	matchHostedZoneId := aws.StringValue(alias.HostedZoneId)

	found, err := describeNetworkLoadBalancers(cloud, request, func(lb *elbv2.LoadBalancer) bool {
		// TODO: Filter by cluster?

		if matchHostedZoneId != aws.StringValue(lb.CanonicalHostedZoneId) {
			return false
		}

		lbDnsName := aws.StringValue(lb.DNSName)
		lbDnsName = strings.TrimSuffix(lbDnsName, ".")
		return lbDnsName == matchDnsName || "dualstack."+lbDnsName == matchDnsName
	})
	if err != nil {
		return nil, fmt.Errorf("error listing NLBs: %v", err)
	}

	if len(found) == 0 {
		return nil, nil
	}

	if len(found) != 1 {
		return nil, fmt.Errorf("Found multiple NLBs with DNSName %q", dnsName)
	}

	return found[0], nil
}

func describeNetworkLoadBalancers(cloud awsup.AWSCloud, request *elbv2.DescribeLoadBalancersInput, filter func(*elbv2.LoadBalancer) bool) ([]*elbv2.LoadBalancer, error) {
	var found []*elbv2.LoadBalancer
	err := cloud.ELBV2().DescribeLoadBalancersPages(request, func(p *elbv2.DescribeLoadBalancersOutput, lastPage bool) (shouldContinue bool) {
		for _, lb := range p.LoadBalancers {
			if filter(lb) {
				found = append(found, lb)
			}
		}

		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error listing NLBs: %v", err)
	}

	return found, nil
}

func (e *NetworkLoadBalancer) getDNSName() *string {
	return e.DNSName
}

func (e *NetworkLoadBalancer) getHostedZoneId() *string {
	return e.HostedZoneId
}

func (e *NetworkLoadBalancer) Find(c *fi.CloudupContext) (*NetworkLoadBalancer, error) {
	cloud := c.T.Cloud.(awsup.AWSCloud)

	allLoadBalancers, err := awsup.ListELBV2LoadBalancers(ctx, cloud)
	if err != nil {
		return nil, err
	}

	latest := awsup.FindLatestELBV2ByNameTag(allLoadBalancers, fi.ValueOf(e.Name))
	if err != nil {
		return nil, err
	}

	// Stash deletions for later
	for _, lb := range allLoadBalancers {
		if lb.NameTag() != fi.ValueOf(e.Name) {
			continue
		}
		if latest != nil && latest.ARN() == lb.ARN() {
			continue
		}

		e.deletions = append(e.deletions, buildDeleteNLB(lb))
	}

	if latest == nil {
		return nil, nil
	}

	lb := latest.LoadBalancer

	loadBalancerArn := latest.ARN()

	actual := &NetworkLoadBalancer{}
	actual.Name = e.Name
	actual.CLBName = e.CLBName
	actual.DNSName = lb.DNSName
	actual.HostedZoneId = lb.CanonicalHostedZoneId // CanonicalHostedZoneNameID
	actual.Scheme = lb.Scheme
	actual.VPC = &VPC{ID: lb.VpcId}
	actual.Type = lb.Type
	actual.IpAddressType = lb.IpAddressType

	// tagMap, err := cloud.DescribeELBV2Tags([]string{loadBalancerArn})
	// if err != nil {
	// 	return nil, err
	// }
	actual.Tags = make(map[string]string)
	for _, tag := range latest.Tags {
		k := aws.StringValue(tag.Key)
		if strings.HasPrefix(k, "aws:cloudformation:") {
			continue
		}
		if k == KopsResourceRevisionTag {
			continue
		}
		actual.Tags[k] = aws.StringValue(tag.Value)
	}

	for _, az := range lb.AvailabilityZones {
		sm := &SubnetMapping{
			Subnet: &Subnet{ID: az.SubnetId},
		}
		for _, a := range az.LoadBalancerAddresses {
			if a.PrivateIPv4Address != nil {
				if sm.PrivateIPv4Address != nil {
					return nil, fmt.Errorf("NLB has more then one PrivateIPv4Address, which is unexpected. This is a bug in kOps, please open a GitHub issue.")
				}
				sm.PrivateIPv4Address = a.PrivateIPv4Address
			}
			if a.AllocationId != nil {
				if sm.AllocationID != nil {
					return nil, fmt.Errorf("NLB has more then one AllocationID per subnet, which is unexpected. This is a bug in kOps, please open a GitHub issue.")
				}
				sm.AllocationID = a.AllocationId
			}
		}
		actual.SubnetMappings = append(actual.SubnetMappings, sm)
	}

	for _, sg := range lb.SecurityGroups {
		actual.SecurityGroups = append(actual.SecurityGroups, &SecurityGroup{ID: sg})
	}

	{
		lbAttributes, err := findNetworkLoadBalancerAttributes(cloud, aws.StringValue(loadBalancerArn))
		if err != nil {
			return nil, err
		}
		klog.V(4).Infof("NLB Load Balancer attributes: %+v", lbAttributes)

		for _, attribute := range lbAttributes {
			if attribute.Value == nil {
				continue
			}
			switch key, value := attribute.Key, attribute.Value; *key {
			case "load_balancing.cross_zone.enabled":
				b, err := strconv.ParseBool(*value)
				if err != nil {
					return nil, err
				}
				actual.CrossZoneLoadBalancing = fi.PtrTo(b)
			case "access_logs.s3.enabled":
				b, err := strconv.ParseBool(*value)
				if err != nil {
					return nil, err
				}
				if actual.AccessLog == nil {
					actual.AccessLog = &NetworkLoadBalancerAccessLog{}
				}
				actual.AccessLog.Enabled = fi.PtrTo(b)
			case "access_logs.s3.bucket":
				if actual.AccessLog == nil {
					actual.AccessLog = &NetworkLoadBalancerAccessLog{}
				}
				if fi.ValueOf(value) != "" {
					actual.AccessLog.S3BucketName = value
				}
			case "access_logs.s3.prefix":
				if actual.AccessLog == nil {
					actual.AccessLog = &NetworkLoadBalancerAccessLog{}
				}
				if fi.ValueOf(value) != "" {
					actual.AccessLog.S3BucketPrefix = value
				}
			default:
				klog.V(2).Infof("unsupported key -- ignoring, %v.\n", key)
			}
		}
	}

	// Avoid spurious mismatches
	if subnetMappingSlicesEqualIgnoreOrder(actual.SubnetMappings, e.SubnetMappings) {
		actual.SubnetMappings = e.SubnetMappings
	}
	if e.DNSName == nil {
		e.DNSName = actual.DNSName
	}
	if e.HostedZoneId == nil {
		e.HostedZoneId = actual.HostedZoneId
	}
	// if e.LoadBalancerName == nil {
	// 	e.LoadBalancerName = actual.LoadBalancerName
	// }

	// An existing internal NLB can't be updated to dualstack.
	if fi.ValueOf(actual.Scheme) == elbv2.LoadBalancerSchemeEnumInternal && fi.ValueOf(actual.IpAddressType) == elbv2.IpAddressTypeIpv4 {
		e.IpAddressType = actual.IpAddressType
	}

	// // We allow for the LoadBalancerName to be wrong:
	// // 1. We don't want to force a rename of the NLB, because that is a destructive operation
	// if fi.ValueOf(e.LoadBalancerName) != fi.ValueOf(actual.LoadBalancerName) {
	// 	klog.V(2).Infof("Reusing existing load balancer with name: %q", aws.StringValue(actual.LoadBalancerName))
	// 	e.LoadBalancerName = actual.LoadBalancerName
	// }

	_ = actual.Normalize(c)
	actual.WellKnownServices = e.WellKnownServices
	actual.Lifecycle = e.Lifecycle
	actual.LoadBalancerBaseName = e.LoadBalancerBaseName

	// Store for other tasks
	e.loadBalancerArn = aws.StringValue(lb.LoadBalancerArn)
	actual.loadBalancerArn = e.loadBalancerArn
	e.revision, _ = latest.GetTag(KopsResourceRevisionTag)
	actual.revision = e.revision

	klog.V(4).Infof("Found NLB %+v", actual)

	// AWS does not allow us to add security groups to an ELB that was initially created without them.
	// This forces a new revision (currently, the only operation that forces a new revision)
	if len(actual.SecurityGroups) == 0 && len(e.SecurityGroups) > 0 {
		klog.Warningf("setting securityGroups on an existing NLB created without securityGroups; will force a new NLB")
		t := time.Now()
		revision := strconv.FormatInt(t.Unix(), 10)
		actual = nil
		e.revision = revision
	}

	return actual, nil
}

var _ fi.HasAddress = &NetworkLoadBalancer{}

// GetWellKnownServices implements fi.HasAddress::GetWellKnownServices.
// It indicates which services we support with this load balancer.
func (e *NetworkLoadBalancer) GetWellKnownServices() []wellknownservices.WellKnownService {
	return e.WellKnownServices
}

func (e *NetworkLoadBalancer) FindAddresses(c *fi.CloudupContext) ([]string, error) {
	ctx := c.Context()

	var addresses []string

	cloud := c.T.Cloud.(awsup.AWSCloud)
	cluster := c.T.Cluster

	allLoadBalancers, err := awsup.ListELBV2LoadBalancers(ctx, cloud)
	if err != nil {
		return nil, err
	}

	lb := awsup.FindLatestELBV2ByNameTag(allLoadBalancers, fi.ValueOf(e.Name))

	if lb != nil {
		if fi.ValueOf(lb.LoadBalancer.DNSName) != "" {
			addresses = append(addresses, fi.ValueOf(lb.LoadBalancer.DNSName))
		}

		if cluster.UsesNoneDNS() {
			nis, err := cloud.FindELBV2NetworkInterfacesByName(fi.ValueOf(e.VPC.ID), aws.StringValue(lb.LoadBalancer.LoadBalancerName))
			if err != nil {
				return nil, fmt.Errorf("failed to find network interfaces matching %q: %w", aws.StringValue(lb.LoadBalancer.LoadBalancerName), err)
			}
			for _, ni := range nis {
				if fi.ValueOf(ni.PrivateIpAddress) != "" {
					addresses = append(addresses, fi.ValueOf(ni.PrivateIpAddress))
				}
			}
		}
	}

	sort.Strings(addresses)

	return addresses, nil
}

func (e *NetworkLoadBalancer) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(e, c)
}

func (e *NetworkLoadBalancer) Normalize(c *fi.CloudupContext) error {
	// We need to sort our arrays consistently, so we don't get spurious changes
	sort.Stable(OrderSubnetMappingsByName(e.SubnetMappings))

	e.IpAddressType = fi.PtrTo("dualstack")
	for _, subnet := range e.SubnetMappings {
		for _, clusterSubnet := range c.T.Cluster.Spec.Networking.Subnets {
			if clusterSubnet.Name == fi.ValueOf(subnet.Subnet.ShortName) && clusterSubnet.IPv6CIDR == "" {
				e.IpAddressType = fi.PtrTo("ipv4")
			}
		}
	}

	return nil
}

func (*NetworkLoadBalancer) CheckChanges(a, e, changes *NetworkLoadBalancer) error {
	if a == nil {
		if fi.ValueOf(e.Name) == "" {
			return fi.RequiredField("Name")
		}
		if len(e.SubnetMappings) == 0 {
			return fi.RequiredField("SubnetMappings")
		}

		if e.CrossZoneLoadBalancing != nil {
			if e.CrossZoneLoadBalancing == nil {
				return fi.RequiredField("CrossZoneLoadBalancing")
			}
		}

		if e.AccessLog != nil {
			if e.AccessLog.Enabled == nil {
				return fi.RequiredField("Accesslog.Enabled")
			}
			if *e.AccessLog.Enabled {
				if e.AccessLog.S3BucketName == nil {
					return fi.RequiredField("Accesslog.S3Bucket")
				}
			}
		}
	} else {
		if len(changes.SubnetMappings) > 0 {
			expectedSubnets := make(map[string]*string)
			for _, s := range e.SubnetMappings {
				if s.AllocationID != nil {
					expectedSubnets[*s.Subnet.ID] = s.AllocationID
				} else if s.PrivateIPv4Address != nil {
					expectedSubnets[*s.Subnet.ID] = s.PrivateIPv4Address
				} else {
					expectedSubnets[*s.Subnet.ID] = nil
				}
			}

			for _, s := range a.SubnetMappings {
				eIP, ok := expectedSubnets[*s.Subnet.ID]
				if !ok {
					return fmt.Errorf("network load balancers do not support detaching subnets")
				}
				if fi.ValueOf(eIP) != fi.ValueOf(s.PrivateIPv4Address) || fi.ValueOf(eIP) != fi.ValueOf(s.AllocationID) {
					return fmt.Errorf("network load balancers do not support modifying address settings")
				}
			}
		}
	}
	return nil
}

func (_ *NetworkLoadBalancer) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *NetworkLoadBalancer) error {
	loadBalancerArn := ""

	revision := e.revision

	// // AWS does not allow us to add security groups to an ELB that was initially created without them.
	// // This forces a new revision (currently, the only operation that forces a new revision)
	// if a != nil && len(a.SecurityGroups) == 0 && len(e.SecurityGroups) > 0 {
	// 	t := time.Now()
	// 	revision = strconv.FormatInt(t.Unix(), 10)

	// 	a = nil
	// }

	// TODO: Use maps.Clone when we are >= go1.21 on supported branches
	tags := make(map[string]string)
	for k, v := range e.Tags {
		tags[k] = v
	}

	if revision != "" {
		tags[KopsResourceRevisionTag] = revision
	}

	// var loadBalancerName string
	// var loadBalancerArn string

	if a == nil {
		if revision != "" {
			tags[KopsResourceRevisionTag] = revision
		}

		createLoadBalancerName := fi.ValueOf(e.LoadBalancerBaseName)
		if revision != "" {
			s := fi.ValueOf(e.LoadBalancerBaseName) + "-" + revision

			// We always compute the hash and add it, lest we trick users into assuming that we never do this
			opt := truncate.TruncateStringOptions{
				MaxLength:     32,
				AlwaysAddHash: true,
				HashLength:    6,
			}
			createLoadBalancerName = truncate.TruncateString(s, opt)
		}

		{
			request := &elbv2.CreateLoadBalancerInput{}
			request.Name = &createLoadBalancerName
			request.Scheme = e.Scheme
			request.Type = e.Type
			request.IpAddressType = e.IpAddressType
			request.Tags = awsup.ELBv2Tags(tags)

			for _, subnetMapping := range e.SubnetMappings {
				request.SubnetMappings = append(request.SubnetMappings, &elbv2.SubnetMapping{
					SubnetId:           subnetMapping.Subnet.ID,
					AllocationId:       subnetMapping.AllocationID,
					PrivateIPv4Address: subnetMapping.PrivateIPv4Address,
				})
			}

			for _, sg := range e.SecurityGroups {
				request.SecurityGroups = append(request.SecurityGroups, sg.ID)
			}

			klog.V(2).Infof("Creating NLB %q", createLoadBalancerName)

			response, err := t.Cloud.ELBV2().CreateLoadBalancer(request)
			if err != nil {
				return fmt.Errorf("error creating NLB %q: %w", createLoadBalancerName, err)
			}
			if len(response.LoadBalancers) != 1 {
				return fmt.Errorf("error creating NLB %q: found %d", createLoadBalancerName, len(response.LoadBalancers))
			}

			lb := response.LoadBalancers[0]
			e.DNSName = lb.DNSName
			e.HostedZoneId = lb.CanonicalHostedZoneId
			e.VPC = &VPC{ID: lb.VpcId}
			loadBalancerArn = aws.StringValue(lb.LoadBalancerArn)
			e.loadBalancerArn = loadBalancerArn
			e.revision = revision
		}

		if e.waitForLoadBalancerReady {
			klog.Infof("Waiting for load balancer %q to be created...", loadBalancerName)
			request := &elbv2.DescribeLoadBalancersInput{
				Names: []*string{&loadBalancerName},
			}

			err := t.Cloud.ELBV2().WaitUntilLoadBalancerAvailableWithContext(ctx, request)
			if err != nil {
				return fmt.Errorf("error waiting for NLB %q: %w", loadBalancerName, err)
			}
		}

	} else {
		// We removed some tags for the diff, but we want to preserve it
		for k, v := range a.Tags {
			if k == KopsResourceRevisionTag {
				tags[KopsResourceRevisionTag] = v
			}
		}

		// loadBalancerName = fi.ValueOf(a.LoadBalancerName)

		// lb, err := findNetworkLoadBalancerByLoadBalancerName(t.Cloud, loadBalancerName)
		// if err != nil {
		// 	return fmt.Errorf("error getting load balancer by name: %v", err)
		// }

		loadBalancerArn = a.loadBalancerArn

		if changes.IpAddressType != nil {
			request := &elbv2.SetIpAddressTypeInput{
				IpAddressType:   e.IpAddressType,
				LoadBalancerArn: &loadBalancerArn,
			}
			if _, err := t.Cloud.ELBV2().SetIpAddressType(request); err != nil {
				return fmt.Errorf("error setting the IP addresses type: %v", err)
			}
		}

		if changes.SubnetMappings != nil {
			actualSubnets := make(map[string]*string)
			for _, s := range a.SubnetMappings {
				// actualSubnets[*s.Subnet.ID] = s
				if s.AllocationID != nil {
					actualSubnets[*s.Subnet.ID] = s.AllocationID
				}
				if s.PrivateIPv4Address != nil {
					actualSubnets[*s.Subnet.ID] = s.PrivateIPv4Address
				}
			}

			var awsSubnetMappings []*elbv2.SubnetMapping
			hasChanges := false
			for _, s := range e.SubnetMappings {
				aIP, ok := actualSubnets[*s.Subnet.ID]
				if !ok || (fi.ValueOf(s.PrivateIPv4Address) != fi.ValueOf(aIP) && fi.ValueOf(s.AllocationID) != fi.ValueOf(aIP)) {
					hasChanges = true
				}
				awsSubnetMappings = append(awsSubnetMappings, &elbv2.SubnetMapping{
					SubnetId:           s.Subnet.ID,
					AllocationId:       s.AllocationID,
					PrivateIPv4Address: s.PrivateIPv4Address,
				})
			}

			if hasChanges {
				request := &elbv2.SetSubnetsInput{}
				request.SetLoadBalancerArn(loadBalancerArn)
				request.SetSubnetMappings(awsSubnetMappings)

				klog.V(2).Infof("Attaching Load Balancer to new subnets")
				if _, err := t.Cloud.ELBV2().SetSubnets(request); err != nil {
					return fmt.Errorf("error attaching load balancer to new subnets: %v", err)
				}
			}
		}

		if changes.SecurityGroups != nil {
			request := &elbv2.SetSecurityGroupsInput{
				LoadBalancerArn: &loadBalancerArn,
			}
			for _, sg := range e.SecurityGroups {
				request.SecurityGroups = append(request.SecurityGroups, sg.ID)
			}

			klog.V(2).Infof("Updating Load Balancer Security Groups")
			if _, err := t.Cloud.ELBV2().SetSecurityGroups(request); err != nil {
				return fmt.Errorf("Error updating security groups on Load Balancer: %v", err)
			}
		}

		if err := t.AddELBV2Tags(loadBalancerArn, tags); err != nil {
			return err
		}

		if err := t.RemoveELBV2Tags(loadBalancerArn, tags); err != nil {
			return err
		}
	}

	if err := e.modifyLoadBalancerAttributes(t, a, e, changes, loadBalancerArn); err != nil {
		klog.Infof("error modifying NLB attributes: %v", err)
		return err
	}
	return nil
}

type terraformNetworkLoadBalancer struct {
	Name                   string                                      `cty:"name"`
	Internal               bool                                        `cty:"internal"`
	Type                   string                                      `cty:"load_balancer_type"`
	IPAddressType          *string                                     `cty:"ip_address_type"`
	SecurityGroups         []*terraformWriter.Literal                  `cty:"security_groups"`
	SubnetMappings         []terraformNetworkLoadBalancerSubnetMapping `cty:"subnet_mapping"`
	CrossZoneLoadBalancing bool                                        `cty:"enable_cross_zone_load_balancing"`
	AccessLog              *terraformNetworkLoadBalancerAccessLog      `cty:"access_logs"`

	Tags map[string]string `cty:"tags"`
}

type terraformNetworkLoadBalancerSubnetMapping struct {
	Subnet             *terraformWriter.Literal `cty:"subnet_id"`
	AllocationID       *string                  `cty:"allocation_id"`
	PrivateIPv4Address *string                  `cty:"private_ipv4_address"`
}

func (_ *NetworkLoadBalancer) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *NetworkLoadBalancer) error {
	nlbTF := &terraformNetworkLoadBalancer{
		Name:                   *e.LoadBalancerBaseName,
		Internal:               fi.ValueOf(e.Scheme) == elbv2.LoadBalancerSchemeEnumInternal,
		Type:                   elbv2.LoadBalancerTypeEnumNetwork,
		Tags:                   e.Tags,
		CrossZoneLoadBalancing: fi.ValueOf(e.CrossZoneLoadBalancing),
	}
	if fi.ValueOf(e.IpAddressType) == "dualstack" {
		nlbTF.IPAddressType = e.IpAddressType
	}

	for _, subnetMapping := range e.SubnetMappings {
		nlbTF.SubnetMappings = append(nlbTF.SubnetMappings, terraformNetworkLoadBalancerSubnetMapping{
			Subnet:             subnetMapping.Subnet.TerraformLink(),
			AllocationID:       subnetMapping.AllocationID,
			PrivateIPv4Address: subnetMapping.PrivateIPv4Address,
		})
	}

	for _, sg := range e.SecurityGroups {
		nlbTF.SecurityGroups = append(nlbTF.SecurityGroups, sg.TerraformLink())
	}
	terraformWriter.SortLiterals(nlbTF.SecurityGroups)

	if e.AccessLog != nil && fi.ValueOf(e.AccessLog.Enabled) {
		nlbTF.AccessLog = &terraformNetworkLoadBalancerAccessLog{
			Enabled:        e.AccessLog.Enabled,
			S3BucketName:   e.AccessLog.S3BucketName,
			S3BucketPrefix: e.AccessLog.S3BucketPrefix,
		}
	}

	err := t.RenderResource("aws_lb", e.TerraformName(), nlbTF)
	if err != nil {
		return err
	}

	return nil
}

func (e *NetworkLoadBalancer) TerraformName() string {
	tfName := strings.Replace(fi.ValueOf(e.Name), ".", "-", -1)
	return tfName
}

func (e *NetworkLoadBalancer) TerraformLink(params ...string) *terraformWriter.Literal {
	prop := "id"
	if len(params) > 0 {
		prop = params[0]
	}
	return terraformWriter.LiteralProperty("aws_lb", e.TerraformName(), prop)
}

// FindDeletions schedules deletion of the corresponding legacy classic load balancer when it no longer has targets.
func (e *NetworkLoadBalancer) FindDeletions(context *fi.CloudupContext) ([]fi.CloudupDeletion, error) {
	var deletions []fi.CloudupDeletion

	deletions = append(deletions, e.deletions...)

	if e.CLBName != nil {
		cloud := context.T.Cloud.(awsup.AWSCloud)

		lb, err := cloud.FindELBByNameTag(fi.ValueOf(e.CLBName))
		if err != nil {
			return nil, err
		}

		if lb != nil {
			// Testing shows that the instances are deregistered immediately after the apply_cluster.
			// TODO: Figure out how to delay deregistration until instances are terminated.
			//if len(lb.Instances) > 0 {
			//	klog.V(2).Infof("CLB %s has targets; not scheduling deletion", *lb.LoadBalancerName)
			//	return nil, nil
			//}

			klog.V(4).Infof("Found CLB %v", aws.StringValue(lb.LoadBalancerName))
			deletions = append(deletions, buildDeleteClassicLoadBalancer(fi.ValueOf(e.CLBName), lb))
		}
	}

	return deletions, nil
}

type deleteClassicLoadBalancer struct {
	fi.CloudupDeletionBase
	obj *elb.LoadBalancerDescription
}

func buildDeleteClassicLoadBalancer(name string, obj *elb.LoadBalancerDescription) *deleteClassicLoadBalancer {
	d := &deleteClassicLoadBalancer{}
	d.obj = obj
	d.Info.Type = "classic-load-balancer"
	d.Info.ID = aws.StringValue(obj.LoadBalancerName)
	d.Info.Name = name
	d.Info.DeferDeletion = false // TODO: Should we defer?
	return d
}

func (d deleteClassicLoadBalancer) Delete(ctx context.Context, t fi.CloudupTarget) error {
	awsTarget, ok := t.(*awsup.AWSAPITarget)
	if !ok {
		return fmt.Errorf("unexpected target type for deletion: %T", t)
	}

	_, err := awsTarget.Cloud.ELB().DeleteLoadBalancerWithContext(ctx, &elb.DeleteLoadBalancerInput{
		LoadBalancerName: d.obj.LoadBalancerName,
	})
	if err != nil {
		return fmt.Errorf("deleting classic LoadBalancer: %w", err)
	}

	return nil
}

// deleteNLB tracks a NLB that we're going to delete
// It implements fi.CloudupDeletion
type deleteNLB struct {
	fi.CloudupDeletionBase
	obj *awsup.LoadBalancerInfo
}

func buildDeleteNLB(obj *awsup.LoadBalancerInfo) *deleteNLB {
	d := &deleteNLB{}
	d.obj = obj
	d.Info.Type = "network-load-balancer"
	d.Info.ID = obj.ARN()
	d.Info.Name = obj.NameTag()
	d.Info.DeferDeletion = true
	return d
}

var _ fi.CloudupDeletion = &deleteNLB{}

func (d *deleteNLB) Delete(ctx context.Context, t fi.CloudupTarget) error {
	awsTarget, ok := t.(*awsup.AWSAPITarget)
	if !ok {
		return fmt.Errorf("unexpected target type for deletion: %T", t)
	}

	arn := d.obj.ARN()
	klog.V(2).Infof("deleting load balancer %q", arn)
	if _, err := awsTarget.Cloud.ELBV2().DeleteLoadBalancerWithContext(ctx, &elbv2.DeleteLoadBalancerInput{
		LoadBalancerArn: &arn,
	}); err != nil {
		return fmt.Errorf("error deleting ELB LoadBalancer %q: %w", arn, err)
	}

	return nil
}
