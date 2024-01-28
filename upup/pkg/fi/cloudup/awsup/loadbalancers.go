package awsup

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"k8s.io/klog/v2"
)

// KopsResourceRevisionTag is the tag used to store the revision timestamp,
// when we are forced to create a new version of a resource because we cannot modify it in-place.
// This happens when the resource field is immutable;
// it also happens for ELBs, when we cannot have two ELBs pointing at the same target group
// and thus must create a second.
const KopsResourceRevisionTag = "kops.k8s.io/revision"

type LoadBalancerInfo struct {
	LoadBalancer *elbv2.LoadBalancer
	Tags         []*elbv2.Tag
}

// ARN returns the ARN of the load balancer.
func (i *LoadBalancerInfo) ARN() string {
	return aws.StringValue(i.LoadBalancer.LoadBalancerArn)
}

// NameTag returns the value of the tag with the key "Name".
func (i *LoadBalancerInfo) NameTag() string {
	s, _ := i.GetTag("Name")
	return s
}

// GetTag returns the value of the tag with the given key.
func (i *LoadBalancerInfo) GetTag(key string) (string, bool) {
	for _, tag := range i.Tags {
		if aws.StringValue(tag.Key) == key {
			return aws.StringValue(tag.Value), true
		}
	}
	return "", false
}

func ListELBV2LoadBalancers(ctx context.Context, cloud AWSCloud) ([]*LoadBalancerInfo, error) {

	// TODO: Any way around this?
	klog.V(2).Infof("Listing all NLBs for ListELBV2LoadBalancers")

	request := &elbv2.DescribeLoadBalancersInput{}
	// ELBV2 DescribeTags has a limit of 20 names, so we set the page size here to 20 also
	request.PageSize = aws.Int64(20)

	byARN := make(map[string]*LoadBalancerInfo)

	var errs []error
	err := cloud.ELBV2().DescribeLoadBalancersPagesWithContext(ctx, request, func(p *elbv2.DescribeLoadBalancersOutput, lastPage bool) bool {
		if len(p.LoadBalancers) == 0 {
			return true
		}

		tagRequest := &elbv2.DescribeTagsInput{}

		for _, elb := range p.LoadBalancers {
			arn := aws.StringValue(elb.LoadBalancerArn)
			byARN[arn] = &LoadBalancerInfo{LoadBalancer: elb}

			// TODO: Any way to filter by cluster here?

			tagRequest.ResourceArns = append(tagRequest.ResourceArns, elb.LoadBalancerArn)
		}

		tagResponse, err := cloud.ELBV2().DescribeTags(tagRequest)
		if err != nil {
			errs = append(errs, fmt.Errorf("listing ELB tags: %w", err))
			return false
		}

		for _, t := range tagResponse.TagDescriptions {
			arn := aws.StringValue(t.ResourceArn)

			info := byARN[arn]
			if info == nil {
				klog.Fatalf("found tag for load balancer we didn't ask for %q", arn)
			}

			info.Tags = append(info.Tags, t.Tags...)
		}

		return true
	})
	if err != nil {
		return nil, fmt.Errorf("listing ELB LoadBalancers: %w", err)
	}
	if len(errs) != 0 {
		return nil, fmt.Errorf("listing ELB LoadBalancers: %w", errors.Join(errs...))
	}

	cloudTags := cloud.Tags()

	var results []*LoadBalancerInfo
	for _, v := range byARN {
		if !MatchesElbV2Tags(cloudTags, v.Tags) {
			continue
		}
		results = append(results, v)
	}
	return results, nil
}

func MatchesElbV2Tags(tags map[string]string, actual []*elbv2.Tag) bool {
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
