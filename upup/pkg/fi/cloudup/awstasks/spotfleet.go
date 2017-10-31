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

package awstasks

import (
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"reflect"
	"strings"
	"k8s.io/apimachinery/pkg/util/sets"
)

//go:generate fitask -type=SpotFleet
type SpotFleet struct {
	Name      *string
	Lifecycle *fi.Lifecycle

	// BidPricePerUnitHour is the bid price per unit hour.
	BidPricePerUnitHour *string

	// TargetCapacity is the number of units to request
	TargetCapacity *int64

	// IAMFleetRole is the IAM role that should be used by spot fleet itself
	IAMFleetRole *IAMRole

	UserData *fi.ResourceHolder

	ImageID *string

	InstanceTypes []string

	SSHKey             *SSHKey
	SecurityGroups     []*SecurityGroup
	AssociatePublicIP  *bool
	IAMInstanceProfile *IAMInstanceProfile

	// RootVolumeSize is the size of the EBS root volume to use, in GB
	RootVolumeSize *int64
	// RootVolumeType is the type of the EBS root volume to use (e.g. gp2)
	RootVolumeType *string
	// RootVolumeIops is the number of IOPS for the root volume, used when volume type is io1
	RootVolumeIops *int64
	// RootVolumeOptimization enables EBS optimization for an instance
	RootVolumeOptimization *bool

	// Tenancy. Can be either default or dedicated.
	Tenancy *string

	Subnets []*Subnet
	Tags    map[string]string

	// id is set once the object exists / is found
	id *string
}

func (e *SpotFleet) Find(c *fi.Context) (*SpotFleet, error) {
	cloud := c.Cloud.(awsup.AWSCloud)

	findTags := cloud.BuildTags(e.Name)

	var matches []*ec2.SpotFleetRequestConfig

	glog.V(2).Infof("listing SpotFleet requests with DescribeSpotFleetRequests")
	request := &ec2.DescribeSpotFleetRequestsInput{}
	err := cloud.EC2().DescribeSpotFleetRequestsPages(request, func(page *ec2.DescribeSpotFleetRequestsOutput, lastPage bool) (shouldContinue bool) {
		for _, r := range page.SpotFleetRequestConfigs {
			if r.SpotFleetRequestConfig == nil {
				continue
			}

			switch aws.StringValue(r.SpotFleetRequestState) {
			case ec2.BatchStateCancelled, ec2.BatchStateCancelledRunning, ec2.BatchStateCancelledTerminating:
				continue
			}

			if len(r.SpotFleetRequestConfig.LaunchSpecifications) == 0 {
				continue
			}
			ls := r.SpotFleetRequestConfig.LaunchSpecifications[0]
			tags := extractInstanceTagMap(ls)
			hasAll := true
			for k, v := range findTags {
				if tags[k] != v {
					hasAll = false
				}
			}

			if hasAll {
				matches = append(matches, r)
			}
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("error listing spot-fleet requests: %v", err)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	if len(matches) > 1 {
		glog.Warningf("found multiple matching spot-fleet requests")
		for _, m := range matches {
			glog.Warningf("found multiple matching spot-fleet requests: %v", m)
		}
		return nil, fmt.Errorf("found multiple matching spot-fleet requests")
	}

	a := matches[0]
	config := a.SpotFleetRequestConfig

	// We collect a few values which we expect to be the same across all LaunchSpecifications
	instanceTypes := sets.NewString()
	subnets := sets.NewString()
	securityGroups := sets.NewString()
	imageIDs := sets.NewString()
	sshKeys := sets.NewString()
	rootVolumeSize := sets.NewInt64()
	rootVolumeType := sets.NewString()
	rootVolumeIops := sets.NewInt64()
	iamInstanceProfiles := sets.NewString()
	userDatas := sets.NewString()
	var ebsOptimized *bool
	var associatePublicIP *bool

	tags := make(map[string]string)
	for _, ls := range config.LaunchSpecifications {
		instanceTypes.Insert(strings.Split(aws.StringValue(ls.InstanceType), ",")...)
		if ls.SubnetId != nil {
			subnets.Insert(strings.Split(aws.StringValue(ls.SubnetId), ",")...)
		}
		for _, sg := range ls.SecurityGroups {
			securityGroups.Insert(aws.StringValue(sg.GroupId))
		}

		imageIDs.Insert(aws.StringValue(ls.ImageId))
		sshKeys.Insert(aws.StringValue(ls.KeyName))
		for _, bdm := range ls.BlockDeviceMappings {
			if bdm.VirtualName != nil {
				// Ephemeral device
				continue
			}
			if bdm.Ebs != nil {
				rootVolumeSize.Insert(aws.Int64Value(bdm.Ebs.VolumeSize))
				rootVolumeType.Insert(aws.StringValue(bdm.Ebs.VolumeType))
				rootVolumeIops.Insert(aws.Int64Value(bdm.Ebs.Iops))
			}
		}

		for _, ni := range ls.NetworkInterfaces {
			if ni.SubnetId != nil {
				subnets.Insert(aws.StringValue(ni.SubnetId))
			}
			if ni.Groups != nil {
				for _, g := range ni.Groups {
					securityGroups.Insert(aws.StringValue(g))
				}
			}
			associatePublicIP = ni.AssociatePublicIpAddress
		}
		ebsOptimized = ls.EbsOptimized

		userData := ""
		if ls.UserData != nil {
			userDataBytes, err := base64.StdEncoding.DecodeString(aws.StringValue(ls.UserData))
			if err != nil {
				glog.Warningf("error decoding userdata from launchspec: %v", err)
				userData = aws.StringValue(ls.UserData)
			} else {
				userData = string(userDataBytes)
			}
		}
		userDatas.Insert(userData)
		if ls.IamInstanceProfile != nil {
			iamInstanceProfiles.Insert(aws.StringValue(ls.IamInstanceProfile.Name))
		}

		// TODO: This logic is particularly "trickable"
		for k, v := range extractInstanceTagMap(ls) {
			tags[k] = v
		}
	}

	actual := &SpotFleet{
		id: a.SpotFleetRequestId,

		Name:                e.Name,
		TargetCapacity:      config.TargetCapacity,
		BidPricePerUnitHour: config.SpotPrice,
		IAMFleetRole:        &IAMRole{ID: config.IamFleetRole},
		InstanceTypes:       instanceTypes.List(),

		RootVolumeOptimization: ebsOptimized,
		AssociatePublicIP:      associatePublicIP,

		Tags: tags,
	}

	for _, subnet := range subnets.List() {
		actual.Subnets = append(actual.Subnets, &Subnet{
			ID: fi.String(subnet),
		})
	}
	for _, sg := range securityGroups.List() {
		actual.SecurityGroups = append(actual.SecurityGroups, &SecurityGroup{
			ID: fi.String(sg),
		})
	}
	if imageIDs.Len() == 1 {
		actual.ImageID = fi.String(imageIDs.List()[0])
	}
	if sshKeys.Len() == 1 {
		actual.SSHKey = &SSHKey{Name: fi.String(sshKeys.List()[0])}
	}
	if rootVolumeSize.Len() == 1 {
		actual.RootVolumeSize = fi.Int64(rootVolumeSize.List()[0])
	}
	if rootVolumeType.Len() == 1 {
		actual.RootVolumeType = fi.String(rootVolumeType.List()[0])
	}
	if rootVolumeIops.Len() == 1 {
		actual.RootVolumeIops = fi.Int64(rootVolumeIops.List()[0])
	}
	if userDatas.Len() == 1 {
		actual.UserData = fi.WrapResource(fi.NewStringResource(userDatas.List()[0]))
	}
	if iamInstanceProfiles.Len() == 1 {
		actual.IAMInstanceProfile = &IAMInstanceProfile{Name: fi.String(iamInstanceProfiles.List()[0])}
	}

	// Avoid spurious changes on ImageId
	if e.ImageID != nil && actual.ImageID != nil && *actual.ImageID != *e.ImageID {
		image, err := cloud.ResolveImage(*e.ImageID)
		if err != nil {
			glog.Warningf("unable to resolve image: %q: %v", *e.ImageID, err)
		} else if image == nil {
			glog.Warningf("unable to resolve image: %q: not found", *e.ImageID)
		} else if aws.StringValue(image.ImageId) == *actual.ImageID {
			glog.V(4).Infof("Returning matching ImageId as expected name: %q -> %q", *actual.ImageID, *e.ImageID)
			actual.ImageID = e.ImageID
		}
	}

	// Prevent spurious mismatches
	actual.Lifecycle = e.Lifecycle

	return actual, nil
}

// extractInstanceTagMap returns the tags that will be applied to an instance, from the provided launch spec
func extractInstanceTagMap(ls *ec2.SpotFleetLaunchSpecification) map[string]string {
	var tags []*ec2.Tag
	for _, tagSpec := range ls.TagSpecifications {
		if aws.StringValue(tagSpec.ResourceType) == "instance" {
			tags = tagSpec.Tags
		}
	}
	tagMap := make(map[string]string)
	for _, t := range tags {
		tagMap[aws.StringValue(t.Key)] = aws.StringValue(t.Value)
	}
	return tagMap
}

func (e *SpotFleet) Run(c *fi.Context) error {
	// This is messy - we don't have the cloud when we're building the tags, so this is the first opportunity to build them
	c.Cloud.(awsup.AWSCloud).AddTags(e.Name, e.Tags)

	return fi.DefaultDeltaRunMethod(e, c)
}

func (s *SpotFleet) CheckChanges(a, e, changes *SpotFleet) error {
	//if a == nil {
	//	// TODO: Create validate method?
	//	if e.SpotFleetTable == nil {
	//		return fi.RequiredField("SpotFleetTable")
	//	}
	//	if e.CIDR == nil {
	//		return fi.RequiredField("CIDR")
	//	}
	//	targetCount := 0
	//	if e.InternetGateway != nil {
	//		targetCount++
	//	}
	//	if e.Instance != nil {
	//		targetCount++
	//	}
	//	if e.NatGateway != nil {
	//		targetCount++
	//	}
	//	if targetCount == 0 {
	//		return fmt.Errorf("InternetGateway or Instance or NatGateway is required")
	//	}
	//	if targetCount != 1 {
	//		return fmt.Errorf("Cannot set more than 1 InternetGateway or Instance or NatGateway")
	//	}
	//}
	//
	//if a != nil {
	//	if changes.SpotFleetTable != nil {
	//		return fi.CannotChangeField("SpotFleetTable")
	//	}
	//	if changes.CIDR != nil {
	//		return fi.CannotChangeField("CIDR")
	//	}
	//}
	return nil
}

func (_ *SpotFleet) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *SpotFleet) error {
	if a == nil {
		var instanceTags []*ec2.Tag
		for k, v := range e.Tags {
			instanceTags = append(instanceTags, &ec2.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}

		imageID := fi.StringValue(e.ImageID)
		image, err := t.Cloud.ResolveImage(imageID)
		if err != nil {
			return fmt.Errorf("unable to resolve image: %q: %v", imageID, err)
		} else if image == nil {
			return fmt.Errorf("unable to resolve image: %q: not found", imageID)
		}

		spotFleetTags := []*ec2.SpotFleetTagSpecification{
			{ResourceType: aws.String("instance"), Tags: instanceTags},
		}

		//var securityGroups []*ec2.GroupIdentifier
		var securityGroupIDs []*string
		for _, sg := range e.SecurityGroups {
			//securityGroups = append(securityGroups, &ec2.GroupIdentifier{
			//	GroupId: sg.ID,
			//})
			securityGroupIDs = append(securityGroupIDs, sg.ID)
		}

		var launchSpecifications []*ec2.SpotFleetLaunchSpecification

		for _, instanceType := range e.InstanceTypes {
			instanceTypeInfo, err := awsup.GetMachineTypeInfo(instanceType)
			if err != nil {
				return err
			}

			// TODO: Validate to prevent non-spot instance types ("Note that T2 and HS1 instance types are not supported.")

			for _, subnet := range e.Subnets {
				// No way to specify AssociatePublicIP false without doing NetworkInterfaces?
				// And that then => single subnet?

				placement := &ec2.SpotPlacement{
					AvailabilityZone: subnet.AvailabilityZone,
					Tenancy:          e.Tenancy,
				}

				launchSpec := &ec2.SpotFleetLaunchSpecification{
					EbsOptimized: e.RootVolumeOptimization,
					ImageId:      image.ImageId,
					InstanceType: aws.String(instanceType),

					// TODO: Enable or disable monitoring for the instances.
					//Monitoring *SpotFleetMonitoring `locationName:"monitoring" type:"structure"`

					// The placement information.
					Placement: placement,

					// The bid price per unit hour for the specified instance type. If this value
					// is not specified, the default is the Spot bid price specified for the fleet.
					// To determine the bid price per unit hour, divide the Spot bid price by the
					// value of WeightedCapacity.
					//SpotPrice *string `locationName:"spotPrice" type:"string"`

					// The tags to apply during creation.
					TagSpecifications: spotFleetTags,

					// The number of units provided by the specified instance type. These are the
					// same units that you chose to set the target capacity in terms (instances
					// or a performance characteristic such as vCPUs, memory, or I/O).
					//
					// If the target capacity divided by this value is not a whole number, we round
					// the number of instances to the next whole number. If this value is not specified,
					// the default is 1.
					//WeightedCapacity *float64 `locationName:"weightedCapacity" type:"double"`
				}

				// For now, we use memory as the sole metric
				// TODO: Reserve some capacity for system overhead - would be pretty cool
				launchSpec.WeightedCapacity = aws.Float64(float64(instanceTypeInfo.MemoryGB))
				ni := &ec2.InstanceNetworkInterfaceSpecification{
					AssociatePublicIpAddress: e.AssociatePublicIP,
					DeleteOnTermination:      aws.Bool(true),
					DeviceIndex:              aws.Int64(0),
					SubnetId:                 subnet.ID,
					Groups:                   securityGroupIDs,
				}
				launchSpec.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{ni}

				if e.IAMInstanceProfile != nil {
					launchSpec.IamInstanceProfile = &ec2.IamInstanceProfileSpecification{
						Name: e.IAMInstanceProfile.Name,
					}
				}

				if e.SSHKey != nil {
					launchSpec.KeyName = e.SSHKey.Name
				}

				if e.UserData != nil {
					d, err := e.UserData.AsBytes()
					if err != nil {
						return fmt.Errorf("error rendering AutoScalingLaunchConfiguration UserData: %v", err)
					}
					launchSpec.UserData = aws.String(base64.StdEncoding.EncodeToString(d))
				}

				{
					rootDevices, err := e.buildRootDevice(t.Cloud)
					if err != nil {
						return err
					}

					ephemeralDevices, err := buildEphemeralDevices(&instanceType)
					if err != nil {
						return err
					}

					if len(rootDevices) != 0 || len(ephemeralDevices) != 0 {
						for deviceName, bdm := range rootDevices {
							launchSpec.BlockDeviceMappings = append(launchSpec.BlockDeviceMappings, bdm.ToEC2(deviceName))
						}
						for deviceName, bdm := range ephemeralDevices {
							launchSpec.BlockDeviceMappings = append(launchSpec.BlockDeviceMappings, bdm.ToEC2(deviceName))
						}
					}
				}

				launchSpecifications = append(launchSpecifications, launchSpec)
			}
		}

		config := &ec2.SpotFleetRequestConfigData{
			// Indicates how to allocate the target capacity across the Spot pools specified
			// by the Spot fleet request. The default is lowestPrice.
			//AllocationStrategy *string `locationName:"allocationStrategy" type:"string" enum:"AllocationStrategy"`

			// A unique, case-sensitive identifier you provide to ensure idempotency of
			// your listings. This helps avoid duplicate listings. For more information,
			// see Ensuring Idempotency (http://docs.aws.amazon.com/AWSEC2/latest/APIReference/Run_Instance_Idempotency.html).
			//ClientToken *string `locationName:"clientToken" type:"string"`

			// Indicates whether running Spot instances should be terminated if the target
			// capacity of the Spot fleet request is decreased below the current size of
			// the Spot fleet.
			ExcessCapacityTerminationPolicy: aws.String("default"),

			// The number of units fulfilled by this request compared to the set target
			// capacity.
			//FulfilledCapacity *float64 `locationName:"fulfilledCapacity" type:"double"`

			// Information about the launch specifications for the Spot fleet request.
			//
			// LaunchSpecifications is a required field
			LaunchSpecifications: launchSpecifications,

			// Indicates whether Spot fleet should replace unhealthy instances.
			ReplaceUnhealthyInstances: aws.Bool(true),

			// The bid price per unit hour.
			//
			// SpotPrice is a required field
			SpotPrice: e.BidPricePerUnitHour,

			// The number of units to request. You can choose to set the target capacity
			// in terms of instances or a performance characteristic that is important to
			// your application workload, such as vCPUs, memory, or I/O.
			//
			// TargetCapacity is a required field
			TargetCapacity: e.TargetCapacity,

			// Indicates whether running Spot instances should be terminated when the Spot
			// fleet request expires.
			TerminateInstancesWithExpiration: aws.Bool(true),

			// The type of request. Indicates whether the fleet will only request the target
			// capacity or also attempt to maintain it. When you request a certain target
			// capacity, the fleet will only place the required bids. It will not attempt
			// to replenish Spot instances if capacity is diminished, nor will it submit
			// bids in alternative Spot pools if capacity is not available. When you want
			// to maintain a certain target capacity, fleet will place the required bids
			// to meet this target capacity. It will also automatically replenish any interrupted
			// instances. Default: maintain.
			//Type *string `locationName:"type" type:"string" enum:"FleetType"`

			// The start date and time of the request, in UTC format (for example, YYYY-MM-DDTHH:MM:SSZ).
			// The default is to start fulfilling the request immediately.
			//ValidFrom *time.Time `locationName:"validFrom" type:"timestamp" timestampFormat:"iso8601"`

			// The end date and time of the request, in UTC format (for example, YYYY-MM-DDTHH:MM:SSZ).
			// At this point, no new Spot instance requests are placed or enabled to fulfill
			// the request.
			//ValidUntil *time.Time `locationName:"validUntil" type:"timestamp" timestampFormat:"iso8601"`
		}

		if e.IAMFleetRole != nil {
			// Grants the Spot fleet permission to terminate Spot instances on your behalf
			// when you cancel its Spot fleet request using CancelSpotFleetRequests or when
			// the Spot fleet request expires, if you set terminateInstancesWithExpiration.
			//
			// IamFleetRole is a required field
			config.IamFleetRole = e.IAMFleetRole.arn
		}

		req := &ec2.RequestSpotFleetInput{
			SpotFleetRequestConfig: config,
		}

		glog.V(2).Infof("Creating SpotFleet %s", aws.StringValue(e.Name))

		glog.V(4).Infof("spot fleet request: %v", req)

		response, err := t.Cloud.EC2().RequestSpotFleet(req)
		if err != nil {
			glog.Warningf("error creating SpotFleet: %s", err.Error())
			return fmt.Errorf("error creating SpotFleet: %v", err)
		}
		e.id = response.SpotFleetRequestId
	} else {
		var copy SpotFleet
		copy = *changes

		// TODO: temporary hack
		copy.IAMFleetRole = nil
		copy.SecurityGroups = nil

		if copy.TargetCapacity != nil {
			glog.V(2).Infof("Modifying SpotFleet %s", aws.StringValue(e.Name))

			req := &ec2.ModifySpotFleetRequestInput{}

			req.TargetCapacity = copy.TargetCapacity
			copy.TargetCapacity = nil

			req.SpotFleetRequestId = a.id

			empty := &SpotFleet{}
			if reflect.DeepEqual(empty, &copy) {
				if _, err := t.Cloud.EC2().ModifySpotFleetRequest(req); err != nil {
					glog.Warningf("error modifying SpotFleet: %s", err.Error())
					return fmt.Errorf("error modifying SpotFleet: %v", err)
				} else {
					return nil
				}
			}
		}

		empty := &SpotFleet{}
		if !reflect.DeepEqual(empty, changes) {
			return fmt.Errorf("cannot apply changes to SpotFleet: %v", changes)
		}
	}

	return nil
}

// buildRootDevice builds a BlockDeviceMapping for the root device
func (e *SpotFleet) buildRootDevice(cloud awsup.AWSCloud) (map[string]*BlockDeviceMapping, error) {
	imageID := fi.StringValue(e.ImageID)
	image, err := cloud.ResolveImage(imageID)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve image: %q: %v", imageID, err)
	} else if image == nil {
		return nil, fmt.Errorf("unable to resolve image: %q: not found", imageID)
	}

	rootDeviceName := aws.StringValue(image.RootDeviceName)

	blockDeviceMappings := make(map[string]*BlockDeviceMapping)

	rootDeviceMapping := &BlockDeviceMapping{
		EbsDeleteOnTermination: aws.Bool(true),
		EbsVolumeSize:          e.RootVolumeSize,
		EbsVolumeType:          e.RootVolumeType,
		EbsVolumeIops:          e.RootVolumeIops,
	}

	blockDeviceMappings[rootDeviceName] = rootDeviceMapping

	return blockDeviceMappings, nil
}

//type terraformSpotFleet struct {
//	SpotFleetTableID  *terraform.Literal `json:"SpotFleet_table_id"`
//	CIDR              *string            `json:"destination_cidr_block,omitempty"`
//	InternetGatewayID *terraform.Literal `json:"gateway_id,omitempty"`
//	NATGatewayID      *terraform.Literal `json:"nat_gateway_id,omitempty"`
//	InstanceID        *terraform.Literal `json:"instance_id,omitempty"`
//}
//
//func (_ *SpotFleet) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *SpotFleet) error {
//	tf := &terraformSpotFleet{
//		CIDR:             e.CIDR,
//		SpotFleetTableID: e.SpotFleetTable.TerraformLink(),
//	}
//
//	if e.InternetGateway == nil && e.NatGateway == nil {
//		return fmt.Errorf("missing target for SpotFleet")
//	} else if e.InternetGateway != nil {
//		tf.InternetGatewayID = e.InternetGateway.TerraformLink()
//	} else if e.NatGateway != nil {
//		tf.NATGatewayID = e.NatGateway.TerraformLink()
//	}
//
//	if e.Instance != nil {
//		tf.InstanceID = e.Instance.TerraformLink()
//	}
//
//	return t.RenderResource("aws_SpotFleet", *e.Name, tf)
//}
//
//type cloudformationSpotFleet struct {
//	SpotFleetTableID  *cloudformation.Literal `json:"SpotFleetTableId"`
//	CIDR              *string                 `json:"DestinationCidrBlock,omitempty"`
//	InternetGatewayID *cloudformation.Literal `json:"GatewayId,omitempty"`
//	NATGatewayID      *cloudformation.Literal `json:"NatGatewayId,omitempty"`
//	InstanceID        *cloudformation.Literal `json:"InstanceId,omitempty"`
//}
//
//func (_ *SpotFleet) RenderCloudformation(t *cloudformation.CloudformationTarget, a, e, changes *SpotFleet) error {
//	tf := &cloudformationSpotFleet{
//		CIDR:             e.CIDR,
//		SpotFleetTableID: e.SpotFleetTable.CloudformationLink(),
//	}
//
//	if e.InternetGateway == nil && e.NatGateway == nil {
//		return fmt.Errorf("missing target for SpotFleet")
//	} else if e.InternetGateway != nil {
//		tf.InternetGatewayID = e.InternetGateway.CloudformationLink()
//	} else if e.NatGateway != nil {
//		tf.NATGatewayID = e.NatGateway.CloudformationLink()
//	}
//
//	if e.Instance != nil {
//		return fmt.Errorf("instance cloudformation SpotFleets not yet implemented")
//		//tf.InstanceID = e.Instance.CloudformationLink()
//	}
//
//	return t.RenderResource("AWS::EC2::SpotFleet", *e.Name, tf)
//}
