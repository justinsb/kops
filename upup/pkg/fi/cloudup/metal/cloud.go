/*
Copyright 2024 The Kubernetes Authors.

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

package metal

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/cloudinstances"
	"k8s.io/kops/upup/pkg/fi"
)

var _ fi.Cloud = &Cloud{}

// Cloud holds the fi.Cloud implementation for metal resources.
type Cloud struct {
	// client *scw.Client
	// region scw.Region
	// zone   scw.Zone
	// dns    dnsprovider.Interface
	// tags   map[string]string

	// domainAPI      *domain.API
	// iamAPI         *iam.API
	// instanceAPI    *instance.API
	// ipamAPI        *ipam.API
	// lbAPI          *lb.ZonedAPI
	// marketplaceAPI *marketplace.API
}

// NewScwCloud returns a Cloud with a Scaleway Client using the env vars SCW_PROFILE or
// SCW_ACCESS_KEY, SCW_SECRET_KEY and SCW_DEFAULT_PROJECT_ID
func NewCloud() (*Cloud, error) {
	// var scwClient *scw.Client
	// var region scw.Region
	// var zone scw.Zone
	// var err error

	// if profileName := os.Getenv("SCW_PROFILE"); profileName == "REDACTED" {
	// 	// If the profile is REDACTED, we're running integration tests so no need for authentication
	// 	scwClient, err = scw.NewClient(scw.WithoutAuth())
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// } else {
	// 	profile, err := CreateValidScalewayProfile()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	scwClient, err = scw.NewClient(
	// 		scw.WithProfile(profile),
	// 		scw.WithUserAgent(KopsUserAgentPrefix+kopsv.Version),
	// 	)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("creating client for Scaleway Cloud: %w", err)
	// 	}
	// 	region = scw.Region(fi.ValueOf(profile.DefaultRegion))
	// 	zone = scw.Zone(fi.ValueOf(profile.DefaultZone))
	// }

	// if tags != nil {
	// 	region, err = scw.ParseRegion(tags["region"])
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	zone, err = scw.ParseZone(tags["zone"])
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// return &scwCloudImplementation{
	// 	client:         scwClient,
	// 	region:         region,
	// 	zone:           zone,
	// 	dns:            dns.NewProvider(domain.NewAPI(scwClient)),
	// 	tags:           tags,
	// 	domainAPI:      domain.NewAPI(scwClient),
	// 	iamAPI:         iam.NewAPI(scwClient),
	// 	instanceAPI:    instance.NewAPI(scwClient),
	// 	ipamAPI:        ipam.NewAPI(scwClient),
	// 	lbAPI:          lb.NewZonedAPI(scwClient),
	// 	marketplaceAPI: marketplace.NewAPI(scwClient),
	// }, nil
	cloud := &Cloud{}
	return cloud, nil
}

func (c *Cloud) ProviderID() kops.CloudProviderID {
	return kops.CloudProviderMetal
}
func (c *Cloud) DNS() (dnsprovider.Interface, error) {
	return nil, fmt.Errorf("method not implemented")
}

// FindVPCInfo looks up the specified VPC by id, returning info if found, otherwise (nil, nil).
func (c *Cloud) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	return nil, fmt.Errorf("method not implemented")
}

// DeleteInstance deletes a cloud instance.
func (c *Cloud) DeleteInstance(instance *cloudinstances.CloudInstance) error {
	return fmt.Errorf("method not implemented")
}

// // DeregisterInstance drains a cloud instance and loadbalancers.
func (c *Cloud) DeregisterInstance(instance *cloudinstances.CloudInstance) error {
	return fmt.Errorf("method not implemented")
}

// DeleteGroup deletes the cloud resources that make up a CloudInstanceGroup, including the instances.
func (c *Cloud) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
	return fmt.Errorf("method not implemented")
}

// DetachInstance causes a cloud instance to no longer be counted against the group's size limits.
func (c *Cloud) DetachInstance(instance *cloudinstances.CloudInstance) error {
	return fmt.Errorf("method not implemented")
}

// GetCloudGroups returns a map of cloud instances that back a kops cluster.
// Detached instances must be returned in the NeedUpdate slice.
func (c *Cloud) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	return nil, fmt.Errorf("method not implemented")
}

// Region returns the cloud region bound to the cloud instance.
// If the region concept does not apply, returns "".
func (c *Cloud) Region() string {
	return ""
}

// FindClusterStatus discovers the status of the cluster, by inspecting the cloud objects
func (c *Cloud) FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error) {
	// etcdStatus, err := findEtcdStatus(c, cluster)
	// if err != nil {
	// 	return nil, err
	// }
	klog.Warningf("method metal.Cloud::FindClusterStatus stub-implemented")
	return &kops.ClusterStatus{
		// EtcdClusters: etcdStatus,
	}, nil

	// return nil, fmt.Errorf("method metal.Cloud::FindClusterStatus not implemented")
}

func (c *Cloud) GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
	// name := "api." + cluster.Name
	var ret []fi.ApiIngressStatus
	ret = append(ret, fi.ApiIngressStatus{
		// Hostname: name,
		IP: "10.78.79.73",
	})
	return ret, nil
}

// func (s *scwCloudImplementation) ClusterName(tags []string) string {
// 	if tags != nil {
// 		return ClusterNameFromTags(tags)
// 	}
// 	if clusterName, ok := s.tags[TagClusterName]; ok {
// 		return clusterName
// 	}
// 	return ""
// }

// func (s *scwCloudImplementation) DNS() (dnsprovider.Interface, error) {
// 	provider, err := dnsprovider.GetDnsProvider(dns.ProviderName, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("error building DNS provider: %w", err)
// 	}
// 	return provider, nil
// }

// func (s *scwCloudImplementation) ProviderID() kops.CloudProviderID {
// 	return kops.CloudProviderScaleway
// }

// func (s *scwCloudImplementation) Region() string {
// 	return string(s.region)
// }

// func (s *scwCloudImplementation) Zone() string {
// 	return string(s.zone)
// }

// func (s *scwCloudImplementation) DomainService() *domain.API {
// 	return s.domainAPI
// }

// func (s *scwCloudImplementation) IamService() *iam.API {
// 	return s.iamAPI
// }

// func (s *scwCloudImplementation) InstanceService() *instance.API {
// 	return s.instanceAPI
// }

// func (s *scwCloudImplementation) IPAMService() *ipam.API {
// 	return s.ipamAPI
// }

// func (s *scwCloudImplementation) LBService() *lb.ZonedAPI {
// 	return s.lbAPI
// }

// func (s *scwCloudImplementation) MarketplaceService() *marketplace.API {
// 	return s.marketplaceAPI
// }

// func (s *scwCloudImplementation) DeleteGroup(group *cloudinstances.CloudInstanceGroup) error {
// 	toDelete := append(group.NeedUpdate, group.Ready...)
// 	for _, cloudInstance := range toDelete {
// 		err := s.DeleteInstance(cloudInstance)
// 		if err != nil {
// 			return fmt.Errorf("error deleting group %q: %w", group.HumanName, err)
// 		}
// 	}
// 	return nil
// }

// func (s *scwCloudImplementation) DeleteInstance(i *cloudinstances.CloudInstance) error {
// 	server, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
// 		Zone:     s.zone,
// 		ServerID: i.ID,
// 	})
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(4).Infof("error deleting cloud instance %s of group %s : instance was already deleted", i.ID, i.CloudInstanceGroup.HumanName)
// 			return nil
// 		}
// 		return fmt.Errorf("deleting cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 	}

// 	err = s.DeleteServer(server.Server)
// 	if err != nil {
// 		return fmt.Errorf("deleting cloud instance %s of group %s: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 	}

// 	return nil
// }

// func (s *scwCloudImplementation) DeregisterInstance(i *cloudinstances.CloudInstance) error {
// 	server, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
// 		Zone:     s.zone,
// 		ServerID: i.ID,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("deregistering cloud instance %s of group %q: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 	}
// 	serverIP, err := s.GetServerIP(server.Server.ID, server.Server.Zone)
// 	if err != nil {
// 		return fmt.Errorf("deregistering cloud instance %s of group %q: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 	}

// 	// We remove the instance's IP from load-balancers
// 	lbs, err := s.GetClusterLoadBalancers(s.ClusterName(server.Server.Tags))
// 	if err != nil {
// 		return fmt.Errorf("deregistering cloud instance %s of group %q: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 	}
// 	for _, loadBalancer := range lbs {
// 		backEnds, err := s.lbAPI.ListBackends(&lb.ZonedAPIListBackendsRequest{
// 			Zone: s.zone,
// 			LBID: loadBalancer.ID,
// 		}, scw.WithAllPages())
// 		if err != nil {
// 			return fmt.Errorf("deregistering cloud instance %s of group %q: listing load-balancer's back-ends for instance creation: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 		}
// 		for _, backEnd := range backEnds.Backends {
// 			for _, ip := range backEnd.Pool {
// 				if ip == serverIP {
// 					_, err := s.lbAPI.RemoveBackendServers(&lb.ZonedAPIRemoveBackendServersRequest{
// 						Zone:      s.zone,
// 						BackendID: backEnd.ID,
// 						ServerIP:  []string{serverIP},
// 					})
// 					if err != nil {
// 						return fmt.Errorf("deregistering cloud instance %s of group %q: removing IP from lb: %w", i.ID, i.CloudInstanceGroup.HumanName, err)
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

// func (s *scwCloudImplementation) DetachInstance(i *cloudinstances.CloudInstance) error {
// 	klog.V(8).Infof("Scaleway DetachInstance is not implemented yet")
// 	return fmt.Errorf("DetachInstance is not implemented yet for Scaleway")
// }

// // FindClusterStatus was used before etcd-manager to check the etcd cluster status and prevent unsupported changes.
// func (s *scwCloudImplementation) FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error) {
// 	klog.V(8).Info("Scaleway FindClusterStatus is not implemented")
// 	return nil, nil
// }

// // FindVPCInfo is not implemented yet, it's only here to satisfy the fi.Cloud interface
// func (s *scwCloudImplementation) FindVPCInfo(id string) (*fi.VPCInfo, error) {
// 	klog.V(8).Info("Scaleway clusters don't have a VPC yet so FindVPCInfo is not implemented")
// 	return nil, fmt.Errorf("FindVPCInfo is not implemented yet for Scaleway")
// }

// func (s *scwCloudImplementation) GetApiIngressStatus(cluster *kops.Cluster) ([]fi.ApiIngressStatus, error) {
// 	var ingresses []fi.ApiIngressStatus
// 	name := "api." + cluster.Name

// 	responseLoadBalancers, err := s.lbAPI.ListLBs(&lb.ZonedAPIListLBsRequest{
// 		Zone: s.zone,
// 		Name: &name,
// 	}, scw.WithAllPages())
// 	if err != nil {
// 		return nil, fmt.Errorf("finding load-balancers: %w", err)
// 	}
// 	if len(responseLoadBalancers.LBs) == 0 {
// 		klog.V(8).Infof("Could not find any load-balancers for cluster %s", cluster.Name)
// 		return nil, nil
// 	}
// 	if len(responseLoadBalancers.LBs) > 1 {
// 		klog.V(4).Infof("More than 1 load-balancer with the name %s was found", name)
// 	}

// 	for _, loadBalancer := range responseLoadBalancers.LBs {
// 		for _, lbIP := range loadBalancer.IP {
// 			ingresses = append(ingresses, fi.ApiIngressStatus{IP: lbIP.IPAddress})
// 		}
// 	}

// 	return ingresses, nil
// }

// func (s *scwCloudImplementation) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
// 	groups := make(map[string]*cloudinstances.CloudInstanceGroup)

// 	nodeMap := cloudinstances.GetNodeMap(nodes, cluster)

// 	serverGroups, err := findServerGroups(s, cluster.Name)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to find server groups: %w", err)
// 	}

// 	for _, ig := range instancegroups {
// 		serverGroup, ok := serverGroups[ig.Name]
// 		if !ok {
// 			if warnUnmatched {
// 				klog.Warningf("Server group %q has no corresponding instance group", ig.Name)
// 			}
// 			continue
// 		}

// 		groups[ig.Name], err = buildCloudGroup(s, ig, serverGroup, nodeMap)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to build cloud group for instance group %q: %w", ig.Name, err)
// 		}
// 	}

// 	return groups, nil
// }

// func findServerGroups(s *scwCloudImplementation, clusterName string) (map[string][]*instance.Server, error) {
// 	servers, err := s.GetClusterServers(clusterName, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	serverGroups := make(map[string][]*instance.Server)
// 	for _, server := range servers {
// 		igName := InstanceGroupNameFromTags(server.Tags)
// 		serverGroups[igName] = append(serverGroups[igName], server)
// 	}

// 	return serverGroups, nil
// }

// func buildCloudGroup(s *scwCloudImplementation, ig *kops.InstanceGroup, sg []*instance.Server, nodeMap map[string]*v1.Node) (*cloudinstances.CloudInstanceGroup, error) {
// 	cloudInstanceGroup := &cloudinstances.CloudInstanceGroup{
// 		HumanName:     ig.Name,
// 		InstanceGroup: ig,
// 		Raw:           sg,
// 		MinSize:       int(fi.ValueOf(ig.Spec.MinSize)),
// 		TargetSize:    int(fi.ValueOf(ig.Spec.MinSize)),
// 		MaxSize:       int(fi.ValueOf(ig.Spec.MaxSize)),
// 	}

// 	for _, server := range sg {
// 		status := cloudinstances.CloudInstanceStatusUpToDate
// 		for _, tag := range server.Tags {
// 			if tag == TagNeedsUpdate {
// 				status = cloudinstances.CloudInstanceStatusNeedsUpdate
// 			}
// 		}
// 		cloudInstance, err := cloudInstanceGroup.NewCloudInstance(server.ID, status, nodeMap[server.ID])
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to create cloud instance for server %s(%s): %w", server.Name, server.ID, err)
// 		}
// 		cloudInstance.State = cloudinstances.State(server.State)
// 		cloudInstance.MachineType = server.CommercialType
// 		cloudInstance.Roles = append(cloudInstance.Roles, InstanceRoleFromTags(server.Tags))
// 		ip, err := s.GetServerIP(server.ID, server.Zone)
// 		if err != nil {
// 			return nil, fmt.Errorf("getting server IP: %w", err)
// 		}
// 		cloudInstance.PrivateIP = ip
// 	}

// 	return cloudInstanceGroup, nil
// }

// func (s *scwCloudImplementation) GetClusterDNSRecords(clusterName string) ([]*domain.Record, error) {
// 	names := strings.SplitN(clusterName, ".", 2)
// 	clusterNameShort := names[0]
// 	domainName := names[1]

// 	records, err := s.domainAPI.ListDNSZoneRecords(&domain.ListDNSZoneRecordsRequest{
// 		DNSZone: domainName,
// 	}, scw.WithAllPages())
// 	if err != nil {
// 		return nil, fmt.Errorf("listing cluster DNS records: %w", err)
// 	}

// 	clusterDNSRecords := []*domain.Record(nil)
// 	for _, record := range records.Records {
// 		if strings.HasSuffix(record.Name, clusterNameShort) {
// 			clusterDNSRecords = append(clusterDNSRecords, record)
// 		}
// 	}
// 	return clusterDNSRecords, nil
// }

// func (s *scwCloudImplementation) GetClusterLoadBalancers(clusterName string) ([]*lb.LB, error) {
// 	loadBalancerName := "api." + clusterName
// 	lbs, err := s.lbAPI.ListLBs(&lb.ZonedAPIListLBsRequest{
// 		Zone: s.zone,
// 		Name: &loadBalancerName,
// 	}, scw.WithAllPages())
// 	if err != nil {
// 		return nil, fmt.Errorf("listing cluster load-balancers: %w", err)
// 	}
// 	return lbs.LBs, nil
// }

// func (s *scwCloudImplementation) GetClusterServers(clusterName string, instanceGroupName *string) ([]*instance.Server, error) {
// 	tags := []string{TagClusterName + "=" + clusterName}
// 	if instanceGroupName != nil {
// 		tags = append(tags, fmt.Sprintf("%s=%s", TagInstanceGroup, *instanceGroupName))
// 	}
// 	request := &instance.ListServersRequest{
// 		Zone: s.zone,
// 		Name: instanceGroupName,
// 		Tags: tags,
// 	}
// 	servers, err := s.instanceAPI.ListServers(request, scw.WithAllPages())
// 	if err != nil {
// 		if instanceGroupName != nil {
// 			return nil, fmt.Errorf("failed to list cluster servers named %q: %w", *instanceGroupName, err)
// 		}
// 		return nil, fmt.Errorf("failed to list cluster servers: %w", err)
// 	}
// 	return servers.Servers, nil
// }

// func (s *scwCloudImplementation) GetClusterSSHKeys(clusterName string) ([]*iam.SSHKey, error) {
// 	clusterSSHKeys := []*iam.SSHKey(nil)
// 	allSSHKeys, err := s.iamAPI.ListSSHKeys(&iam.ListSSHKeysRequest{}, scw.WithAllPages())
// 	for _, sshkey := range allSSHKeys.SSHKeys {
// 		if strings.HasPrefix(sshkey.Name, fmt.Sprintf("kubernetes.%s-", clusterName)) {
// 			clusterSSHKeys = append(clusterSSHKeys, sshkey)
// 		}
// 	}
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list cluster ssh keys: %w", err)
// 	}
// 	return clusterSSHKeys, nil
// }

// func (s *scwCloudImplementation) GetClusterVolumes(clusterName string) ([]*instance.Volume, error) {
// 	volumes, err := s.instanceAPI.ListVolumes(&instance.ListVolumesRequest{
// 		Zone: s.zone,
// 		Tags: []string{TagClusterName + "=" + clusterName},
// 	}, scw.WithAllPages())
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to list cluster volumes: %w", err)
// 	}
// 	return volumes.Volumes, nil
// }

// func (s *scwCloudImplementation) GetServerIP(serverID string, zone scw.Zone) (string, error) {
// 	region, err := zone.Region()
// 	if err != nil {
// 		return "", fmt.Errorf("converting zone %s to region: %w", zone, err)
// 	}

// 	ips, err := s.ipamAPI.ListIPs(&ipam.ListIPsRequest{
// 		Region:     region,
// 		IsIPv6:     fi.PtrTo(false),
// 		ResourceID: &serverID,
// 		Zonal:      fi.PtrTo(zone.String()),
// 	}, scw.WithAllPages())
// 	if err != nil {
// 		return "", fmt.Errorf("listing IPs for server %s: %w", serverID, err)
// 	}

// 	if len(ips.IPs) < 1 {
// 		return "", fmt.Errorf("could not find IP for server %s", serverID)
// 	}
// 	if len(ips.IPs) > 1 {
// 		klog.V(10).Infof("Found more than 1 IP for server %s, using %s", serverID, ips.IPs[0].Address.IP.String())
// 	}

// 	return ips.IPs[0].Address.IP.String(), nil
// }

// func (s *scwCloudImplementation) DeleteDNSRecord(record *domain.Record, clusterName string) error {
// 	domainName := strings.SplitN(clusterName, ".", 2)[1]
// 	recordDeleteRequest := &domain.UpdateDNSZoneRecordsRequest{
// 		DNSZone: domainName,
// 		Changes: []*domain.RecordChange{
// 			{
// 				Delete: &domain.RecordChangeDelete{
// 					ID: scw.StringPtr(record.ID),
// 				},
// 			},
// 		},
// 	}
// 	_, err := s.domainAPI.UpdateDNSZoneRecords(recordDeleteRequest)
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(8).Infof("DNS record %q (%s) was already deleted", record.Name, record.ID)
// 			return nil
// 		}
// 		return fmt.Errorf("failed to delete record %s: %w", record.Name, err)
// 	}
// 	return nil
// }

// func (s *scwCloudImplementation) DeleteLoadBalancer(loadBalancer *lb.LB) error {
// 	ipsToRelease := loadBalancer.IP

// 	// We delete the load-balancer once it's in a stable state
// 	_, err := s.lbAPI.WaitForLb(&lb.ZonedAPIWaitForLBRequest{
// 		LBID: loadBalancer.ID,
// 		Zone: s.zone,
// 	})
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(8).Infof("Load-balancer %q (%s) was already deleted", loadBalancer.Name, loadBalancer.ID)
// 			return nil
// 		}
// 		return fmt.Errorf("waiting for load-balancer: %w", err)
// 	}
// 	err = s.lbAPI.DeleteLB(&lb.ZonedAPIDeleteLBRequest{
// 		Zone: s.zone,
// 		LBID: loadBalancer.ID,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("deleting load-balancer %s: %w", loadBalancer.ID, err)
// 	}

// 	// We wait for the load-balancer to be deleted, then we detach its IPs
// 	_, err = s.lbAPI.WaitForLb(&lb.ZonedAPIWaitForLBRequest{
// 		LBID: loadBalancer.ID,
// 		Zone: s.zone,
// 	})
// 	if !is404Error(err) {
// 		return fmt.Errorf("waiting for load-balancer %s after deletion: %w", loadBalancer.ID, err)
// 	}
// 	for _, ip := range ipsToRelease {
// 		err := s.lbAPI.ReleaseIP(&lb.ZonedAPIReleaseIPRequest{
// 			Zone: s.zone,
// 			IPID: ip.ID,
// 		})
// 		if err != nil {
// 			return fmt.Errorf("deleting load-balancer IP: %w", err)
// 		}
// 	}
// 	return nil
// }

// func (s *scwCloudImplementation) DeleteServer(server *instance.Server) error {
// 	srv, err := s.instanceAPI.GetServer(&instance.GetServerRequest{
// 		Zone:     s.zone,
// 		ServerID: server.ID,
// 	})
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(4).Infof("delete server %s: instance %q was already deleted", server.ID, server.Name)
// 			return nil
// 		}
// 		return err
// 	}

// 	// We detach the etcd volumes
// 	for _, volume := range srv.Server.Volumes {
// 		volumeResponse, err := s.instanceAPI.GetVolume(&instance.GetVolumeRequest{
// 			Zone:     s.zone,
// 			VolumeID: volume.ID,
// 		})
// 		if err != nil {
// 			return fmt.Errorf("delete server %s: getting infos for volume %s", server.ID, volume.ID)
// 		}
// 		for _, tag := range volumeResponse.Volume.Tags {
// 			if strings.HasPrefix(tag, TagNameEtcdClusterPrefix) {
// 				_, err = s.instanceAPI.DetachVolume(&instance.DetachVolumeRequest{
// 					Zone:     s.zone,
// 					VolumeID: volume.ID,
// 				})
// 				if err != nil {
// 					return fmt.Errorf("delete server %s: detaching volume %s", server.ID, volume.ID)
// 				}
// 			}
// 		}
// 	}

// 	// We terminate the server. This stops and deletes the machine immediately
// 	_, err = s.instanceAPI.ServerAction(&instance.ServerActionRequest{
// 		Zone:     s.zone,
// 		ServerID: server.ID,
// 		Action:   instance.ServerActionTerminate,
// 	})
// 	if err != nil && !is404Error(err) {
// 		return fmt.Errorf("delete server %s: terminating instance: %w", server.ID, err)
// 	}

// 	_, err = s.instanceAPI.WaitForServer(&instance.WaitForServerRequest{
// 		ServerID: server.ID,
// 		Zone:     s.zone,
// 	})
// 	if err != nil && !is404Error(err) {
// 		return fmt.Errorf("delete server %s: waiting for instance after termination: %w", server.ID, err)
// 	}

// 	return nil
// }

// func (s *scwCloudImplementation) DeleteSSHKey(sshkey *iam.SSHKey) error {
// 	err := s.iamAPI.DeleteSSHKey(&iam.DeleteSSHKeyRequest{
// 		SSHKeyID: sshkey.ID,
// 	})
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(8).Infof("SSH key %q (%s) was already deleted", sshkey.Name, sshkey.ID)
// 			return nil
// 		}
// 		return fmt.Errorf("failed to delete ssh key %s: %w", sshkey.ID, err)
// 	}
// 	return nil
// }

// func (s *scwCloudImplementation) DeleteVolume(volume *instance.Volume) error {
// 	err := s.instanceAPI.DeleteVolume(&instance.DeleteVolumeRequest{
// 		VolumeID: volume.ID,
// 		Zone:     s.zone,
// 	})
// 	if err != nil {
// 		if is404Error(err) {
// 			klog.V(8).Infof("Volume %q (%s) was already deleted", volume.Name, volume.ID)
// 			return nil
// 		}
// 		return fmt.Errorf("failed to delete volume %s: %w", volume.ID, err)
// 	}

// 	_, err = s.instanceAPI.WaitForVolume(&instance.WaitForVolumeRequest{
// 		VolumeID: volume.ID,
// 		Zone:     s.zone,
// 	})
// 	if !is404Error(err) {
// 		return fmt.Errorf("delete volume %s: error waiting for volume after deletion: %w", volume.ID, err)
// 	}

// 	return nil
// }
