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

package clusterapi

import (
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kops "k8s.io/kops/pkg/apis/kops"
	kopsv1alpha2 "k8s.io/kops/pkg/apis/kops/v1alpha2"
	"k8s.io/kops/pkg/apis/nodeup"
	"k8s.io/kops/pkg/assets"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/kopscodecs"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/model/gcemodel"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
)

type Builder struct {
	Clientset simple.Clientset
}

func (b *Builder) BuildMachineDeployment(clusterObj *kopsv1alpha2.Cluster, igObj *kopsv1alpha2.InstanceGroup) ([]*unstructured.Unstructured, error) {
	cluster := &kops.Cluster{}
	{
		if err := kopscodecs.Scheme.Convert(clusterObj, cluster, nil); err != nil {
			return nil, fmt.Errorf("error converting cluster to internal form: %v", err)
		}
	}

	ig := &kops.InstanceGroup{}
	{
		if err := kopscodecs.Scheme.Convert(igObj, ig, nil); err != nil {
			return nil, fmt.Errorf("error converting InstanceGroup to internal form: %v", err)
		}
	}

	cloud, err := cloudup.BuildCloud(cluster)
	if err != nil {
		return nil, err
	}

	gceCloud := cloud.(gce.GCECloud)
	region := gceCloud.Region()

	phase := cloudup.PhaseCluster
	assetBuilder := assets.NewAssetBuilder(cluster, string(phase))

	applyCmd := &cloudup.ApplyClusterCmd{
		Cluster:        cluster,
		Clientset:      b.Clientset,
		InstanceGroups: []*kops.InstanceGroup{ig},
		Phase:          phase,
	}

	if err := applyCmd.AddFileAssets(assetBuilder); err != nil {
		return nil, fmt.Errorf("error adding assets: %v", err)
	}

	nodeupConfig, err := applyCmd.BuildNodeUpConfig(assetBuilder, ig)
	if err != nil {
		return nil, fmt.Errorf("error building nodeup config: %v", err)
	}

	/*
		if !ig.IsMaster() {
			nodeupConfig.ProtokubeImage = nil
			nodeupConfig.Channels = nil
		}
			nodeupConfig.ConfigBase = fi.String("/etc/kubernetes/bootstrap")
	*/

	bootstrapScript := model.BootstrapScript{}

	nodeupAsset, err := cloudup.NodeUpAsset(assetBuilder)
	if err != nil {
		return nil, err
	}
	bootstrapScript.NodeUpSource = strings.Join(nodeupAsset.Locations, ",")
	bootstrapScript.NodeUpSourceHash = nodeupAsset.Hash.Hex()
	bootstrapScript.NodeUpConfigBuilder = func(ig *kops.InstanceGroup) (*nodeup.Config, error) {
		return nodeupConfig, err
	}

	script, err := bootstrapScript.ResourceNodeUp(ig, cluster)
	if err != nil {
		return nil, fmt.Errorf("error building bootstrap script: %v", err)
	}

	scriptString, err := script.AsString()
	if err != nil {
		return nil, fmt.Errorf("error building bootstrap script: %v", err)
	}

	/*
		file := &DataFile{}
		file.Header.Name = "bootstrap.sh"
		file.Header.Size = int64(len(scriptBytes))
		file.Header.Mode = 0755
		file.Data = scriptBytes
		files = append(files, file)
	*/

	//klog.Infof("script %s", string(scriptBytes))

	gce := &gcemodel.GCEModelContext{
		KopsModelContext: &model.KopsModelContext{},
	}
	gce.Cluster = cluster
	gce.Region = region

	// TODO: We should move this into the gcemodel package, and reuse the existing logic
	volumeSize, err := gce.GetVolumeSize(ig)
	if err != nil {
		return nil, err
	}
	// TODO: Support volumeType
	//volumeType := gce.GetVolumeType(ig)

	/*
		disks := []map[string]interface{}{
			{
				"initializeParams": map[string]interface{}{
					"diskSizeGb": volumeSize,
					"diskType":   volumeType,
				},
			},
		}
	*/

	// TODO	CanIPForward
	//t.CanIPForward = fi.Bool(false)

	subnetwork := gce.NameForIPAliasSubnet()
	subnetwork = "regions/" + region + "/subnetworks/" + subnetwork

	networkInterfaces := []map[string]interface{}{}

	if gce.UsesIPAliases() {
		ni := map[string]interface{}{
			"subnetwork": subnetwork,
		}
		var aliasIPRanges []map[string]interface{}
		for k, v := range gce.NodeAliasIPRanges() {
			r := make(map[string]interface{})
			r["ipCidrRange"] = v
			r["subnetworkRangeName"] = k
			aliasIPRanges = append(aliasIPRanges, r)
		}
		ni["aliasIpRanges"] = aliasIPRanges
		networkInterfaces = append(networkInterfaces, ni)
	}

	additionalMetadata := []map[string]interface{}{
		{
			"key":   "cluster-name",
			"value": cluster.Name,
		},
	}

	zones, err := gce.FindZonesForInstanceGroup(ig)
	if err != nil {
		return nil, err
	}
	zone := ""
	if len(zones) == 1 {
		zone = zones[0]
	} else if len(zones) < 1 {
		return nil, fmt.Errorf("must specify zone for GCE")
	} else {
		return nil, fmt.Errorf("cannot specify multiple zones for GCE")
	}

	machineType := ig.Spec.MachineType

	additionalNetworkTags := []string{}
	roles := []string{}
	switch ig.Spec.Role {
	case kops.InstanceGroupRoleNode:
		additionalNetworkTags = append(additionalNetworkTags, gce.GCETagForRole(kops.InstanceGroupRoleNode))
		roles = append(roles, "Node")

	default:
		return nil, fmt.Errorf("unsupported role %q", ig.Spec.Role)
	}

	email := "default"
	serviceAccount := map[string]interface{}{
		"email": email,
		// TODO: scopes?
	}

	image := ig.Spec.Image
	// Expand known short-forms
	// TODO: Move this logic into GCP provider
	{
		tokens := strings.Split(image, "/")
		if len(tokens) == 2 {
			image = "projects/" + tokens[0] + "/global/images/" + tokens[1]
		}
	}

	// TODO: The GCPMachine name doesn't match the Machine name, which is just confusing.

	/*
		providerSpec := map[string]interface{}{
			"apiVersion":        "gceproviderconfig/v1alpha1",
			"kind":              "GCEProviderConfig",
			"roles":             roles,
			"zone":              zone,
			"machineType":       machineType,
			"networkInterfaces": networkInterfaces,
			"disks":             disks,
			"image":             image,
			"instanceTags":      instanceTags,
			"serviceAccounts":   serviceAccounts,
			"instanceMetadata":  instanceMetadata,
		}
	*/

	var machineTemplate *unstructured.Unstructured
	{
		u := &unstructured.Unstructured{}

		u.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha2")
		u.SetKind("GCPMachineTemplate")
		u.SetName(ig.Name)
		u.SetNamespace(ig.Namespace)

		// TODO: We should support instances on GCP without a public IP
		// (I think we need to set up a NAT gateway)
		publicIP := true

		template := map[string]interface{}{
			"spec": map[string]interface{}{
				"instanceType":          machineType,
				"zone":                  zone,
				"image":                 image,
				"rootDeviceSize":        volumeSize,
				"serviceAccount":        serviceAccount,
				"publicIP":              &publicIP,
				"additionalNetworkTags": additionalNetworkTags,
				"additionalMetadata":    additionalMetadata,
			},
		}

		spec := map[string]interface{}{
			"template": template,
		}

		u.Object["spec"] = spec
		machineTemplate = u
	}

	// For reasons not entirely clear, we resolve the cluster from the machine using this label
	// TODO: Use owner refs if available?
	clusterName := clusterObj.Name
	clusterName = strings.ReplaceAll(clusterName, ".", "-")

	// TODO: GCP shouldn't use the cluster name for the GCP resources ... what about sanitization

	// TODO: Should the provider encode this?  Not clear whether the data is supposed to be pre-encoded.
	bootstrapData := base64.StdEncoding.EncodeToString([]byte(scriptString))

	// TODO: GCP provider requires this, but actually only requires it when we aren't using a custom image
	version := "1.16.0"

	var machineDeployment *unstructured.Unstructured
	{
		u := &unstructured.Unstructured{}

		u.SetAPIVersion("cluster.x-k8s.io/v1alpha2")
		u.SetKind("MachineDeployment")
		u.SetName(ig.Name)
		u.SetNamespace(ig.Namespace)

		// TODO: We need MaxSize & MinSize?
		replicas := ig.Spec.MaxSize

		labels := map[string]string{
			"kops.k8s.io/instancegroup":     ig.Name,
			"cluster.x-k8s.io/cluster-name": clusterName,
		}

		template := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"bootstrap": map[string]interface{}{
					"data": bootstrapData,
				},
				"infrastructureRef": map[string]interface{}{
					"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha2",
					"kind":       "GCPMachineTemplate",
					"name":       ig.Name,
					"namespace":  ig.Namespace,
				},
				"version": version,
			},
		}

		spec := map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": labels,
			},
			"template": template,
		}

		u.Object["spec"] = spec

		machineDeployment = u
	}

	return []*unstructured.Unstructured{machineTemplate, machineDeployment}, nil
}
