/*
Copyright 2022 The Kubernetes Authors.

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

package hetznertasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/wellknownservices"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/hetzner"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraformWriter"
)

// +kops:fitask
type LoadBalancer struct {
	Name      *string
	Lifecycle fi.Lifecycle
	Network   *Network

	ID       *int
	Location string
	Type     string
	Services []*LoadBalancerService
	Target   string

	Labels map[string]string
}

var _ fi.CompareWithID = &LoadBalancer{}

func (v *LoadBalancer) CompareWithID() *string {
	return fi.PtrTo(strconv.Itoa(fi.ValueOf(v.ID)))
}

var _ fi.HasAddress = &LoadBalancer{}

// GetWellKnownServices indicates which services we support with this load balancer.
func (v *LoadBalancer) GetWellKnownServices() []wellknownservices.WellKnownService {
	return []wellknownservices.WellKnownService{wellknownservices.KubeAPIServer}
}

func (v *LoadBalancer) FindAddresses(c *fi.CloudupContext) ([]string, error) {
	// TODO(hakman): Use mock to handle this more gracefully
	if strings.HasPrefix(c.T.ClusterConfigBase.Path(), "memfs://tests/") {
		return nil, nil
	}

	ctx := context.TODO()
	cloud := c.T.Cloud.(hetzner.HetznerCloud)
	client := cloud.LoadBalancerClient()

	// TODO(hakman): Find using label selector
	loadbalancers, err := client.All(ctx)
	if err != nil {
		return nil, err
	}

	for _, loadbalancer := range loadbalancers {
		if loadbalancer.Name == fi.ValueOf(v.Name) {
			var addresses []string
			if loadbalancer.PublicNet.IPv4.IP == nil {
				return nil, fmt.Errorf("failed to find load-balancer %q public address", fi.ValueOf(v.Name))
			}
			addresses = append(addresses, loadbalancer.PublicNet.IPv4.IP.String())
			for _, privateNetwork := range loadbalancer.PrivateNet {
				if privateNetwork.IP == nil {
					return nil, fmt.Errorf("failed to find load-balancer %q private address", fi.ValueOf(v.Name))
				}
				addresses = append(addresses, privateNetwork.IP.String())
			}
			return addresses, nil
		}
	}

	return nil, nil
}

func (v *LoadBalancer) Find(c *fi.CloudupContext) (*LoadBalancer, error) {
	ctx := context.TODO()
	cloud := c.T.Cloud.(hetzner.HetznerCloud)
	client := cloud.LoadBalancerClient()

	// TODO(hakman): Find using label selector
	loadbalancers, err := client.All(ctx)
	if err != nil {
		return nil, err
	}

	for _, loadbalancer := range loadbalancers {
		if loadbalancer.Name == fi.ValueOf(v.Name) {
			matches := &LoadBalancer{
				Lifecycle: v.Lifecycle,
				Name:      fi.PtrTo(loadbalancer.Name),
				ID:        fi.PtrTo(loadbalancer.ID),
				Labels:    loadbalancer.Labels,
			}

			if loadbalancer.Location != nil {
				matches.Location = loadbalancer.Location.Name
			}
			if loadbalancer.LoadBalancerType != nil {
				matches.Type = loadbalancer.LoadBalancerType.Name
			}

			for _, service := range loadbalancer.Services {
				loadbalancerService := LoadBalancerService{
					Protocol:        string(service.Protocol),
					ListenerPort:    fi.PtrTo(service.ListenPort),
					DestinationPort: fi.PtrTo(service.DestinationPort),
				}
				matches.Services = append(matches.Services, &loadbalancerService)
			}

			// Put the services into a stable order
			sort.Slice(matches.Services, func(i, j int) bool {
				return fi.ValueOf(matches.Services[i].DestinationPort) < fi.ValueOf(matches.Services[j].ListenerPort)
			})

			for _, target := range loadbalancer.Targets {
				if target.Type == hcloud.LoadBalancerTargetTypeLabelSelector && target.LabelSelector != nil {
					matches.Target = target.LabelSelector.Selector
				}
			}

			// TODO: The API only returns the network ID, a new API call is required to get the network name
			matches.Network = v.Network

			v.ID = matches.ID
			return matches, nil
		}
	}

	return nil, nil
}

func (v *LoadBalancer) Run(c *fi.CloudupContext) error {
	return fi.CloudupDefaultDeltaRunMethod(v, c)
}

func (_ *LoadBalancer) CheckChanges(a, e, changes *LoadBalancer) error {
	if a != nil {
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
		if changes.ID != nil {
			return fi.CannotChangeField("ID")
		}
		if changes.Location != "" {
			return fi.CannotChangeField("Location")
		}
		if changes.Type != "" {
			return fi.CannotChangeField("Type")
		}
		if len(changes.Services) > 0 {
			for _, aService := range a.Services {
				aServiceJSON, err := json.Marshal(aService)
				if err != nil {
					return err
				}
				found := false
				for _, eService := range e.Services {
					eServiceJSON, err := json.Marshal(eService)
					if err != nil {
						return err
					}
					if bytes.Equal(aServiceJSON, eServiceJSON) {
						found = true
					}
				}
				if !found {
					klog.Infof("cannot remove service %v", string(aServiceJSON))
					return fi.CannotChangeField("Services")
				}
			}
		}
		if changes.Target != "" && a.Target != "" {
			return fi.CannotChangeField("Target")
		}
	} else {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
		if e.Location == "" {
			return fi.RequiredField("Location")
		}
		if e.Type == "" {
			return fi.RequiredField("Type")
		}
		if len(e.Services) == 0 {
			return fi.RequiredField("Services")
		}
		if e.Target == "" {
			return fi.RequiredField("Target")
		}
	}
	return nil
}

func (_ *LoadBalancer) RenderHetzner(t *hetzner.HetznerAPITarget, a, e, changes *LoadBalancer) error {
	ctx := context.TODO()
	actionClient := t.Cloud.ActionClient()
	client := t.Cloud.LoadBalancerClient()

	if a == nil {
		if e.Network == nil {
			return fmt.Errorf("failed to find network for loadbalancer %q", fi.ValueOf(e.Name))
		}

		networkID, err := strconv.Atoi(fi.ValueOf(e.Network.ID))
		if err != nil {
			return fmt.Errorf("failed to convert network ID %q to int: %w", fi.ValueOf(e.Network.ID), err)
		}

		opts := hcloud.LoadBalancerCreateOpts{
			Name: fi.ValueOf(e.Name),
			LoadBalancerType: &hcloud.LoadBalancerType{
				Name: e.Type,
			},
			Algorithm: &hcloud.LoadBalancerAlgorithm{
				Type: hcloud.LoadBalancerAlgorithmTypeRoundRobin,
			},
			Location: &hcloud.Location{
				Name: e.Location,
			},
			Labels: e.Labels,
			Targets: []hcloud.LoadBalancerCreateOptsTarget{
				{
					Type: hcloud.LoadBalancerTargetTypeLabelSelector,
					LabelSelector: hcloud.LoadBalancerCreateOptsTargetLabelSelector{
						Selector: e.Target,
					},
					UsePrivateIP: fi.PtrTo(true),
				},
			},
			Network: &hcloud.Network{
				ID: networkID,
			},
		}

		for _, service := range e.Services {
			opts.Services = append(opts.Services, hcloud.LoadBalancerCreateOptsService{
				Protocol:        hcloud.LoadBalancerServiceProtocol(service.Protocol),
				ListenPort:      service.ListenerPort,
				DestinationPort: service.DestinationPort,
			})
		}

		result, _, err := client.Create(ctx, opts)
		if err != nil {
			return err
		}
		_, errCh := actionClient.WatchProgress(ctx, result.Action)
		if err := <-errCh; err != nil {
			return err
		}

	} else {
		var err error
		loadbalancer, _, err := client.Get(ctx, strconv.Itoa(fi.ValueOf(a.ID)))
		if err != nil {
			return err
		}

		// Update the labels
		if changes.Name != nil || len(changes.Labels) != 0 {
			_, _, err := client.Update(ctx, loadbalancer, hcloud.LoadBalancerUpdateOpts{
				Name:   fi.ValueOf(e.Name),
				Labels: e.Labels,
			})
			if err != nil {
				return err
			}
		}

		// Update the services
		if len(changes.Services) > 0 {
			for _, service := range e.Services {
				eServiceJSON, err := json.Marshal(service)
				if err != nil {
					return err
				}

				found := false
				for _, aService := range a.Services {
					aServiceJSON, err := json.Marshal(aService)
					if err != nil {
						return err
					}

					if bytes.Equal(aServiceJSON, eServiceJSON) {
						found = true
					}
				}

				if !found {
					action, _, err := client.AddService(ctx, loadbalancer, hcloud.LoadBalancerAddServiceOpts{
						Protocol:        hcloud.LoadBalancerServiceProtocol(service.Protocol),
						ListenPort:      service.ListenerPort,
						DestinationPort: service.DestinationPort,
					})
					if err != nil {
						return err
					}
					_, errCh := actionClient.WatchProgress(ctx, action)
					if err := <-errCh; err != nil {
						return err
					}
				}
			}
		}

		// Update the targets
		if a.Target == "" {
			action, _, err := client.AddLabelSelectorTarget(ctx, loadbalancer, hcloud.LoadBalancerAddLabelSelectorTargetOpts{
				Selector:     e.Target,
				UsePrivateIP: fi.PtrTo(true),
			})
			if err != nil {
				return err
			}
			_, errCh := actionClient.WatchProgress(ctx, action)
			if err := <-errCh; err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadBalancerService represents a LoadBalancer's service.
type LoadBalancerService struct {
	Protocol        string
	ListenerPort    *int
	DestinationPort *int
}

var _ fi.CloudupHasDependencies = &LoadBalancerService{}

func (e *LoadBalancerService) GetDependencies(tasks map[string]fi.CloudupTask) []fi.CloudupTask {
	return nil
}

type terraformLoadBalancer struct {
	Name     *string                      `cty:"name"`
	Type     *string                      `cty:"load_balancer_type"`
	Location *string                      `cty:"location"`
	Target   *terraformLoadBalancerTarget `cty:"target"`
	Network  *terraformWriter.Literal     `cty:"network"`
	Labels   map[string]string            `cty:"labels"`
}

type terraformLoadBalancerNetwork struct {
	LoadBalancerID *terraformWriter.Literal `cty:"load_balancer_id"`
	NetworkID      *terraformWriter.Literal `cty:"network_id"`
}

type terraformLoadBalancerService struct {
	LoadBalancerID  *terraformWriter.Literal `cty:"load_balancer_id"`
	Protocol        *string                  `cty:"protocol"`
	ListenPort      *int                     `cty:"listen_port"`
	DestinationPort *int                     `cty:"destination_port"`
}

type terraformLoadBalancerTarget struct {
	LoadBalancerID *terraformWriter.Literal `cty:"load_balancer_id"`
	Type           *string                  `cty:"type"`
	LabelSelector  *string                  `cty:"label_selector"`
	UsePrivateIP   *bool                    `cty:"use_private_ip"`
}

func (_ *LoadBalancer) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *LoadBalancer) error {
	{
		tf := &terraformLoadBalancer{
			Name:     e.Name,
			Type:     &e.Type,
			Location: &e.Location,
			Labels:   e.Labels,
		}

		err := t.RenderResource("hcloud_load_balancer", *e.Name, tf)
		if err != nil {
			return err
		}
	}

	{
		tf := &terraformLoadBalancerNetwork{
			LoadBalancerID: e.TerraformLink(),
			NetworkID:      e.Network.TerraformLink(),
		}

		err := t.RenderResource("hcloud_load_balancer_network", *e.Name, tf)
		if err != nil {
			return err
		}
	}

	for _, service := range e.Services {
		tf := &terraformLoadBalancerService{
			LoadBalancerID:  e.TerraformLink(),
			Protocol:        fi.PtrTo(service.Protocol),
			ListenPort:      service.ListenerPort,
			DestinationPort: service.DestinationPort,
		}

		err := t.RenderResource("hcloud_load_balancer_service", fmt.Sprintf("%s-%s-%d", *e.Name, service.Protocol, *service.ListenerPort), tf)
		if err != nil {
			return err
		}
	}

	{
		tf := &terraformLoadBalancerTarget{
			LoadBalancerID: e.TerraformLink(),
			Type:           fi.PtrTo(string(hcloud.LoadBalancerTargetTypeLabelSelector)),
			LabelSelector:  fi.PtrTo(e.Target),
			UsePrivateIP:   fi.PtrTo(true),
		}

		err := t.RenderResource("hcloud_load_balancer_target", *e.Name, tf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *LoadBalancer) TerraformLink() *terraformWriter.Literal {
	return terraformWriter.LiteralProperty("hcloud_load_balancer", *e.Name, "id")
}
