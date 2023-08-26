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

package azuretasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-05-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/wellknownports"
	"k8s.io/kops/pkg/wellknownservices"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/azure"
)

// LoadBalancer is an Azure Cloud LoadBalancer
// +kops:fitask
type LoadBalancer struct {
	Name          *string
	Lifecycle     fi.Lifecycle
	ResourceGroup *ResourceGroup
	Subnet        *Subnet

	// External is set to true when the loadbalancer is used for external traffic
	External *bool

	Tags map[string]*string

	// WellKnownServices indicates which services are supported by this resource (internal only)
	WellKnownServices []wellknownservices.WellKnownService
}

var (
	_ fi.CloudupTask          = &LoadBalancer{}
	_ fi.CompareWithID        = &LoadBalancer{}
	_ fi.CloudupTaskNormalize = &LoadBalancer{}
)

// CompareWithID returns the Name of the LoadBalancer
func (lb *LoadBalancer) CompareWithID() *string {
	return lb.Name
}

// GetWellKnownServices indicates which services we support with this load balancer.
func (lb *LoadBalancer) GetWellKnownServices() []wellknownservices.WellKnownService {
	return lb.WellKnownServices
}

func (lb *LoadBalancer) FindAddresses(c *fi.CloudupContext) ([]string, error) {
	cloud := c.T.Cloud.(azure.AzureCloud)
	loadbalancer, err := cloud.LoadBalancer().Get(context.TODO(), *lb.ResourceGroup.Name, *lb.Name)
	if err != nil && !strings.Contains(err.Error(), "NotFound") {
		return nil, err
	}

	if loadbalancer != nil && loadbalancer.FrontendIPConfigurations != nil && len(*loadbalancer.FrontendIPConfigurations) > 0 {
		var addresses []string
		for _, fipc := range *loadbalancer.FrontendIPConfigurations {
			if fipc.PrivateIPAddress != nil {
				addresses = append(addresses, *fipc.PrivateIPAddress)
			}
			if fipc.PublicIPAddress != nil && fipc.PublicIPAddress.IPAddress != nil {
				addresses = append(addresses, *fipc.PublicIPAddress.IPAddress)
			}
		}
		return addresses, nil
	}

	return nil, nil
}

// Find discovers the LoadBalancer in the cloud provider
func (lb *LoadBalancer) Find(c *fi.CloudupContext) (*LoadBalancer, error) {
	cloud := c.T.Cloud.(azure.AzureCloud)
	l, err := cloud.LoadBalancer().List(context.TODO(), *lb.ResourceGroup.Name)
	if err != nil {
		return nil, err
	}
	var found *network.LoadBalancer
	for _, v := range l {
		if *v.Name == *lb.Name {
			found = &v
			break
		}
	}
	if found == nil {
		return nil, nil
	}

	lbProperties := found.LoadBalancerPropertiesFormat

	feConfigs := *lbProperties.FrontendIPConfigurations
	if len(feConfigs) != 1 {
		return nil, fmt.Errorf("unexpected number of frontend configs found for LoadBalancer %s: %d", *lb.Name, len(feConfigs))
	}
	feConfig := feConfigs[0]
	subnet := feConfig.FrontendIPConfigurationPropertiesFormat.Subnet

	actual := &LoadBalancer{
		Name:              lb.Name,
		Lifecycle:         lb.Lifecycle,
		WellKnownServices: lb.WellKnownServices,
		ResourceGroup: &ResourceGroup{
			Name: lb.ResourceGroup.Name,
		},
		External: to.BoolPtr(feConfig.FrontendIPConfigurationPropertiesFormat.PublicIPAddress != nil),
		Tags:     found.Tags,
	}
	if subnet != nil {
		actual.Subnet = &Subnet{
			Name: subnet.Name,
		}
	}

	return actual, nil
}

func (lb *LoadBalancer) Normalize(c *fi.CloudupContext) error {
	c.T.Cloud.(azure.AzureCloud).AddClusterTags(lb.Tags)
	return nil
}

// Run implements fi.Task.Run.
func (lb *LoadBalancer) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(lb, c)
}

// CheckChanges returns an error if a change is not allowed.
func (*LoadBalancer) CheckChanges(a, e, changes *LoadBalancer) error {
	if a == nil {
		// Check if required fields are set when a new resource is created.
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		return nil
	}

	// Check if unchanegable fields won't be changed.
	if changes.Name != nil {
		return fi.CannotChangeField("Name")
	}
	return nil
}

// RenderAzure creates or updates a Loadbalancer.
func (*LoadBalancer) RenderAzure(t *azure.AzureAPITarget, a, e, changes *LoadBalancer) error {
	if a == nil {
		klog.Infof("Creating a new Loadbalancer with name: %s", fi.ValueOf(e.Name))
	} else {
		klog.Infof("Updating a Loadbalancer with name: %s", fi.ValueOf(e.Name))
	}

	idPrefix := fmt.Sprintf("subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network", t.Cloud.SubscriptionID(), *e.ResourceGroup.Name)
	feConfigProperties := &network.FrontendIPConfigurationPropertiesFormat{}
	if *e.External {
		feConfigProperties.PublicIPAddress = &network.PublicIPAddress{
			ID: to.StringPtr(fmt.Sprintf("/%s/publicIPAddresses/%s", idPrefix, *e.Name)),
		}
	} else {
		feConfigProperties.PrivateIPAllocationMethod = network.Dynamic
		feConfigProperties.Subnet = &network.Subnet{
			ID: to.StringPtr(fmt.Sprintf("/%s/virtualNetworks/%s/subnets/%s", idPrefix, *e.Subnet.VirtualNetwork.Name, *e.Subnet.Name)),
		}
	}
	// TODO: Move hardcoded values to the model
	lb := network.LoadBalancer{
		Location: to.StringPtr(t.Cloud.Region()),
		Sku: &network.LoadBalancerSku{
			Name: network.LoadBalancerSkuNameStandard,
		},
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name:                                    to.StringPtr("LoadBalancerFrontEnd"),
					FrontendIPConfigurationPropertiesFormat: feConfigProperties,
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("LoadBalancerBackEnd"),
				},
			},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr("Health-TCP-443"),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(wellknownports.KubeAPIServer),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
				{
					Name: to.StringPtr("Health-TCP-3988"),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(wellknownports.KopsControllerPort),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr("TCP-443"),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(wellknownports.KubeAPIServer),
						BackendPort:          to.Int32Ptr(wellknownports.KubeAPIServer),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     to.BoolPtr(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/frontendIPConfigurations/%s", idPrefix, *e.Name, *to.StringPtr("LoadBalancerFrontEnd"))),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/backendAddressPools/%s", idPrefix, *e.Name, *to.StringPtr("LoadBalancerBackEnd"))),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/probes/%s", idPrefix, *e.Name, *to.StringPtr("Health-TCP-443"))),
						},
					},
				},
				{
					Name: to.StringPtr("TCP-3988"),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(wellknownports.KopsControllerPort),
						BackendPort:          to.Int32Ptr(wellknownports.KopsControllerPort),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     to.BoolPtr(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/frontendIPConfigurations/%s", idPrefix, *e.Name, *to.StringPtr("LoadBalancerFrontEnd"))),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/backendAddressPools/%s", idPrefix, *e.Name, *to.StringPtr("LoadBalancerBackEnd"))),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/loadbalancers/%s/probes/%s", idPrefix, *e.Name, *to.StringPtr("Health-TCP-3988"))),
						},
					},
				},
			},
		},
		Tags: e.Tags,
	}

	return t.Cloud.LoadBalancer().CreateOrUpdate(
		context.TODO(),
		*e.ResourceGroup.Name,
		*e.Name,
		lb)
}
