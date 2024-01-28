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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"k8s.io/klog/v2"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraformWriter"
)

// +kops:fitask
type NetworkLoadBalancerListener struct {
	// We use the Name tag to find the existing NLB, because we are (more or less) unrestricted when
	// it comes to tag values, but the LoadBalancerName is length limited
	Name      *string
	Lifecycle fi.Lifecycle

	NetworkLoadBalancer *NetworkLoadBalancer

	Port             int
	TargetGroup      *TargetGroup
	SSLCertificateID string
	SSLPolicy        string

	listenerArn string
}

var _ fi.CompareWithID = &NetworkLoadBalancerListener{}
var _ fi.CloudupTaskNormalize = &NetworkLoadBalancerListener{}

func (e *NetworkLoadBalancerListener) CompareWithID() *string {
	return e.Name
}

// var _ fi.CloudupHasDependencies = &NetworkLoadBalancerListener{}

// func (e *NetworkLoadBalancerListener) GetDependencies(tasks map[string]fi.CloudupTask) []fi.CloudupTask {
// 	return nil
// }

// // OrderListenersByPort implements sort.Interface for []OrderListenersByPort, based on port number
// type OrderListenersByPort []*NetworkLoadBalancerListener

// func (a OrderListenersByPort) Len() int      { return len(a) }
// func (a OrderListenersByPort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
// func (a OrderListenersByPort) Less(i, j int) bool {
// 	return a[i].Port < a[j].Port
// }

func (e *NetworkLoadBalancerListener) Find(c *fi.CloudupContext) (*NetworkLoadBalancerListener, error) {
	ctx := c.Context()

	cloud := c.T.Cloud.(awsup.AWSCloud)

	if e.NetworkLoadBalancer == nil {
		return nil, fi.RequiredField("NetworkLoadBalancer")
	}

	loadBalancerArn := e.NetworkLoadBalancer.loadBalancerArn
	if loadBalancerArn == "" {
		return nil, nil
	}

	var l *elbv2.Listener
	{
		request := &elbv2.DescribeListenersInput{
			LoadBalancerArn: &loadBalancerArn,
		}
		// TODO: Move to lbInfo?
		var allListeners []*elbv2.Listener
		if err := cloud.ELBV2().DescribeListenersPagesWithContext(ctx, request, func(page *elbv2.DescribeListenersOutput, lastPage bool) bool {
			allListeners = append(allListeners, page.Listeners...)
			return true
		}); err != nil {
			return nil, fmt.Errorf("error querying for NLB listeners :%v", err)
		}

		var matches []*elbv2.Listener
		for _, listener := range allListeners {
			if aws.Int64Value(listener.Port) == int64(e.Port) {
				matches = append(matches, listener)
			}
		}
		if len(matches) == 0 {
			return nil, nil
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("found multiple listeners matching %+v", e)
		}
		l = matches[0]
	}

	actual := &NetworkLoadBalancerListener{}
	actual.listenerArn = aws.StringValue(l.ListenerArn)

	actual.Port = int(aws.Int64Value(l.Port))
	if len(l.Certificates) != 0 {
		actual.SSLCertificateID = aws.StringValue(l.Certificates[0].CertificateArn) // What if there is more then one certificate, can we just grab the default certificate? we don't set it as default, we only set the one.
		if l.SslPolicy != nil {
			actual.SSLPolicy = aws.StringValue(l.SslPolicy)
		}
	}

	// This will need to be rearranged when we recognized multiple listeners and target groups per NLB
	if len(l.DefaultActions) > 0 {
		targetGroupARN := l.DefaultActions[0].TargetGroupArn
		if targetGroupARN != nil {
			// targetGroupName, err := awsup.GetTargetGroupNameFromARN(fi.ValueOf(targetGroupARN))
			// if err != nil {
			// 	return nil, err
			// }
			// actual.TargetGroups = append(actual.TargetGroups, &TargetGroup{ARN: targetGroupARN, Name: fi.PtrTo(targetGroupName)})

			// cloud := c.T.Cloud.(awsup.AWSCloud)
			// descResp, err := cloud.ELBV2().DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			// 	TargetGroupArns: []*string{targetGroupARN},
			// })
			// if err != nil {
			// 	return nil, fmt.Errorf("error querying for NLB listener target groups: %v", err)
			// }
			// if len(descResp.TargetGroups) != 1 {
			// 	return nil, fmt.Errorf("unexpected DescribeTargetGroups response: %v", descResp)
			// }

			// actual.TargetGroupName = aws.StringValue(descResp.TargetGroups[0].TargetGroupName)

			actual.TargetGroup = &TargetGroup{
				ARN: targetGroupARN,
			}
		}
	}

	// sort.Stable(OrderTargetGroupsByName(actual.TargetGroups))

	_ = actual.Normalize(c)
	actual.Lifecycle = e.Lifecycle

	// Avoid spurious changes
	actual.Name = e.Name
	actual.NetworkLoadBalancer = e.NetworkLoadBalancer

	klog.V(4).Infof("Found NLB listener %+v", actual)

	return actual, nil
}

func (e *NetworkLoadBalancerListener) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(e, c)
}

func (e *NetworkLoadBalancerListener) Normalize(c *fi.CloudupContext) error {
	return nil
}

func (*NetworkLoadBalancerListener) CheckChanges(a, e, changes *NetworkLoadBalancerListener) error {
	return nil
}

func (*NetworkLoadBalancerListener) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *NetworkLoadBalancerListener) error {

	if e.NetworkLoadBalancer == nil {
		return fi.RequiredField("NetworkLoadBalancer")
	}
	loadBalancerArn := e.NetworkLoadBalancer.loadBalancerArn
	if loadBalancerArn == "" {
		return fmt.Errorf("load balancer not yet created (arn not set)")
	}

	// loadBalancerName := fi.ValueOf(e.LoadBalancerBaseName)
	// if loadBalancerName == "" {
	// 	return fi.RequiredField("LoadBalancerName")
	// }

	// revision := ""

	// // TODO: Use maps.Clone when we are >= go1.21 on supported branches
	// tags := make(map[string]string)
	// for k, v := range e.Tags {
	// 	tags[k] = v
	// }

	// if revision != "" {
	// 	tags["Revision"] = revision
	// }

	if a != nil {
		// TODO: Can we do better here?

		// delete the listener before recreating it
		_, err := t.Cloud.ELBV2().DeleteListener(&elbv2.DeleteListenerInput{
			ListenerArn: &a.listenerArn,
		})
		if err != nil {
			return fmt.Errorf("error deleting load balancer listener with arn=%q: %w", e.listenerArn, err)
		}
		a = nil
	}

	if a == nil {

		if e.TargetGroup == nil {
			return fi.RequiredField("TargetGroup")
		}
		targetGroupARN := fi.ValueOf(e.TargetGroup.ARN)
		if targetGroupARN == "" {
			return fmt.Errorf("target group not yet created (arn not set)")
		}
		request := &elbv2.CreateListenerInput{
			DefaultActions: []*elbv2.Action{
				{
					TargetGroupArn: aws.String(targetGroupARN),
					Type:           aws.String(elbv2.ActionTypeEnumForward),
				},
			},
			LoadBalancerArn: aws.String(loadBalancerArn),
			Port:            aws.Int64(int64(e.Port)),
		}

		if e.SSLCertificateID != "" {
			request.Certificates = []*elbv2.Certificate{}
			request.Certificates = append(request.Certificates, &elbv2.Certificate{
				CertificateArn: aws.String(e.SSLCertificateID),
			})
			request.Protocol = aws.String(elbv2.ProtocolEnumTls)
			if e.SSLPolicy != "" {
				request.SslPolicy = aws.String(e.SSLPolicy)
			}
		} else {
			request.Protocol = aws.String(elbv2.ProtocolEnumTcp)
		}

		klog.V(2).Infof("Creating Listener for NLB with port %v", e.Port)
		_, err := t.Cloud.ELBV2().CreateListener(request)
		if err != nil {
			return fmt.Errorf("creating listener for NLB on port %v: %w", e.Port, err)
		}

	}

	// // TODO: Tags?
	// 	if err := t.AddELBV2Tags(loadBalancerArn, tags); err != nil {
	// 		return err
	// 	}

	// 	if err := t.RemoveELBV2Tags(loadBalancerArn, tags); err != nil {
	// 		return err
	// 	}

	return nil
}

type terraformNetworkLoadBalancerListener struct {
	LoadBalancer   *terraformWriter.Literal                     `cty:"load_balancer_arn"`
	Port           int64                                        `cty:"port"`
	Protocol       string                                       `cty:"protocol"`
	CertificateARN *string                                      `cty:"certificate_arn"`
	SSLPolicy      *string                                      `cty:"ssl_policy"`
	DefaultAction  []terraformNetworkLoadBalancerListenerAction `cty:"default_action"`
}

type terraformNetworkLoadBalancerListenerAction struct {
	Type           string                   `cty:"type"`
	TargetGroupARN *terraformWriter.Literal `cty:"target_group_arn"`
}

func (_ *NetworkLoadBalancerListener) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *NetworkLoadBalancerListener) error {
	if e.TargetGroup == nil {
		return fi.RequiredField("TargetGroup")
	}
	// var listenerTG *TargetGroup
	// for _, tg := range e.TargetGroups {
	// 	if aws.StringValue(tg.Name) == listener.TargetGroupName {
	// 		listenerTG = tg
	// 		break
	// 	}
	// }
	// if listenerTG == nil {
	// 	return fmt.Errorf("target group not found for NLB listener %+v", e)
	// }
	listenerTF := &terraformNetworkLoadBalancerListener{
		LoadBalancer: e.NetworkLoadBalancer.TerraformLink(),
		Port:         int64(e.Port),
		DefaultAction: []terraformNetworkLoadBalancerListenerAction{
			{
				Type:           elbv2.ActionTypeEnumForward,
				TargetGroupARN: e.TargetGroup.TerraformLink(),
			},
		},
	}
	if e.SSLCertificateID != "" {
		listenerTF.CertificateARN = &e.SSLCertificateID
		listenerTF.Protocol = elbv2.ProtocolEnumTls
		if e.SSLPolicy != "" {
			listenerTF.SSLPolicy = &e.SSLPolicy
		}
	} else {
		listenerTF.Protocol = elbv2.ProtocolEnumTcp
	}

	err := t.RenderResource("aws_lb_listener", e.TerraformName(), listenerTF)
	if err != nil {
		return err
	}

	return nil
}

func (e *NetworkLoadBalancerListener) TerraformName() string {
	tfName := fmt.Sprintf("%v-%v", e.NetworkLoadBalancer.TerraformName(), e.Port)
	return tfName
}

// func (e *NetworkLoadBalancerListener) TerraformLink(params ...string) *terraformWriter.Literal {
// 	prop := "id"
// 	if len(params) > 0 {
// 		prop = params[0]
// 	}
// 	return terraformWriter.LiteralProperty("aws_lb_listener", e.TerraformName(), prop)
// }
