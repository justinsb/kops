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

package server

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/nodeup"
	"k8s.io/kops/pkg/bootstrap"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/wellknownservices"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"sigs.k8s.io/yaml"
)

func (s *Server) getNodeConfig(ctx context.Context, req *nodeup.BootstrapRequest, identity *bootstrap.VerifyResult) (*nodeup.NodeConfig, error) {
	klog.Infof("getting node config for %+v", req)

	instanceGroupName := identity.InstanceGroupName
	if instanceGroupName == "" {
		return nil, fmt.Errorf("did not find InstanceGroup for node %q", identity.NodeName)
	}

	clusterName := s.opt.ClusterName
	cluster, err := s.clientset.GetCluster(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster %q: %w", clusterName, err)
	}

	nodeConfig := &nodeup.NodeConfig{}

	// Note: For now, we're assuming there is only a single cluster, and it is ours.
	// We therefore use the configured base path

	{
		// p := s.configBase.Join("igconfig", "node", instanceGroupName, "nodeupconfig.yaml")

		// b, err := p.ReadFile(ctx)
		// if err != nil {
		// 	return nil, fmt.Errorf("error loading NodeupConfig %q: %v", p, err)
		// }
		b := &nodeupConfigBuilder{
			clientset: s.clientset,
			cluster:   cluster,
		}

		cloud, err := cloudup.BuildCloud(cluster)
		if err != nil {
			return nil, fmt.Errorf("building cloud: %w", err)
		}
		b.cloud = cloud

		nodeInfo, err := b.buildNodeConfig(ctx, instanceGroupName)
		if err != nil {
			return nil, fmt.Errorf("building nodeup config: %w", err)
		}

		nodeupConfigBytes, err := yaml.Marshal(nodeInfo.NodeupConfig)
		if err != nil {
			return nil, fmt.Errorf("error converting nodeup config to yaml: %w", err)
		}

		nodeConfig.NodeupConfig = string(nodeupConfigBytes)
	}

	{
		secretIDs := []string{
			"dockerconfig",
		}
		nodeConfig.NodeSecrets = make(map[string][]byte)
		for _, id := range secretIDs {
			secret, err := s.secretStore.FindSecret(id)
			if err != nil {
				return nil, fmt.Errorf("error loading secret %q: %w", id, err)
			}
			if secret != nil && secret.Data != nil {
				nodeConfig.NodeSecrets[id] = secret.Data
			}
		}
	}

	return nodeConfig, nil
}

type nodeupConfigBuilder struct {
	clientset simple.Clientset
	cluster   *kops.Cluster
	cloud     fi.Cloud
}

type nodeConfig struct {
	NodeupConfig *nodeup.Config
	bootConfig   *nodeup.BootConfig
}

func (b *nodeupConfigBuilder) buildNodeConfig(ctx context.Context, instanceGroupName string) (*nodeConfig, error) {

	// if b.ClusterName == "" {
	// 	return fmt.Errorf("cluster is required")
	// }
	// if options.InstanceGroup == "" {
	// 	return fmt.Errorf("instance-group is required")
	// }

	// cluster, err := b.clientset.GetCluster(ctx, options.ClusterName)
	// if err != nil {
	// 	return err
	// }
	// if cluster == nil {
	// 	return fmt.Errorf("cluster not found %q", options.ClusterName)
	// }

	cluster := b.cluster
	clientset := b.clientset
	cloud := b.cloud

	// channel, err := cloudup.ChannelForCluster(clientset.VFSContext(), cluster)
	// if err != nil {
	// 	return fmt.Errorf("getting channel for cluster %q: %w", options.ClusterName, err)
	// }

	// cloud, err := cloudup.BuildCloud(cluster)
	// if err != nil {
	// 	return fmt.Errorf("building cloud: %w", err)
	// }

	// instanceGroupList, err := clientset.InstanceGroupsFor(cluster).List(ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return err
	// }

	apply := &cloudup.ApplyClusterCmd{
		Cloud:      cloud,
		Cluster:    cluster,
		Clientset:  clientset,
		DryRun:     true,
		TargetName: cloudup.TargetDryRun,
		GetAssets:  true,
	}
	applyResults, err := apply.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("error during apply: %w", err)
	}

	// fileAssets := &nodemodel.FileAssets{Cluster: cluster}
	// if err := fileAssets.AddFileAssets(applyResults.AssetBuilder); err != nil {
	// 	return nil, err
	// }

	// // assets := make(map[architectures.Architecture][]*assets.MirroredAsset)
	// configBuilder, err := nodemodel.NewNodeUpConfigBuilder(cluster, applyResults.AssetBuilder, fileAssets.Assets, applyResults)
	// if err != nil {
	// 	return err
	// }

	nodeupConfigBuilder := applyResults.NodeUpConfigBuilder
	var instanceGroup *kops.InstanceGroup

	for _, ig := range applyResults.InstanceGroups {
		if ig.Name == instanceGroupName {
			instanceGroup = ig
		}
	}
	if instanceGroup == nil {
		return nil, fmt.Errorf("instance group %q not found", instanceGroupName)
	}

	wellKnownAddresses := make(model.WellKnownAddresses)
	{
		ingresses, err := cloud.GetApiIngressStatus(cluster)
		if err != nil {
			return nil, fmt.Errorf("error getting ingress status: %v", err)
		}

		for _, ingress := range ingresses {
			// TODO: Do we need to support hostnames?
			// if ingress.Hostname != "" {
			// 	apiserverAdditionalIPs = append(apiserverAdditionalIPs, ingress.Hostname)
			// }
			if ingress.IP != "" {
				wellKnownAddresses[wellknownservices.KubeAPIServer] = append(wellKnownAddresses[wellknownservices.KubeAPIServer], ingress.IP)
			}
		}

		if len(wellKnownAddresses[wellknownservices.KubeAPIServer]) == 0 {
			// TODO: Should we support DNS?
			return nil, fmt.Errorf("unable to determine IP address for kube-apiserver")
		}

		for k := range wellKnownAddresses {
			sort.Strings(wellKnownAddresses[k])
		}
	}

	keysets := make(map[string]*fi.Keyset)
	{
		keystore, err := clientset.KeyStore(cluster)
		if err != nil {
			return nil, err
		}

		keyNames := []string{"kubernetes-ca", "etcd-clients-ca"}
		if instanceGroup.HasAPIServer() {
			keyNames = append(keyNames, "etcd-clients-ca")
		}

		for _, etcdCluster := range cluster.Spec.EtcdClusters {
			k := etcdCluster.Name
			keyNames = append(keyNames, "etcd-manager-ca-"+k, "etcd-peers-ca-"+k)
			if k != "events" && k != "main" {
				keyNames = append(keyNames, "etcd-clients-ca-"+k)
			}
		}

		if instanceGroup.HasAPIServer() {
			keyNames = append(keyNames, "apiserver-aggregator-ca", "service-account", "etcd-clients-ca")
		}

		if instanceGroup.IsBastion() {
			keyNames = nil
		}

		for _, keyName := range keyNames {
			keyset, err := keystore.FindKeyset(ctx, keyName)
			if err != nil {
				return nil, fmt.Errorf("getting keyset %q: %w", keyName, err)
			}

			if keyset == nil {
				return nil, fmt.Errorf("failed to find keyset %q", keyName)
			}

			keysets[keyName] = keyset
		}
	}

	nodeupConfig, bootConfig, err := nodeupConfigBuilder.BuildConfig(instanceGroup, wellKnownAddresses, keysets)
	if err != nil {
		return nil, err
	}

	return &nodeConfig{
		NodeupConfig: nodeupConfig,
		bootConfig:   bootConfig,
	}, nil

}
