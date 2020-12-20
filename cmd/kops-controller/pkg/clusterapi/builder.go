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

package clusterapi

import (
	"bytes"
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/registry"
	kopsv1alpha2 "k8s.io/kops/pkg/apis/kops/v1alpha2"
	"k8s.io/kops/pkg/assets"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/kopscodecs"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kops/upup/pkg/fi/secrets"
)

type Builder struct {
	Clientset simple.Clientset
}

// ClusterAPIBuilder is implemented by the model task that knows how to map an InstanceGroup to k8s objects
type ClusterAPIBuilder interface {
	MapToClusterAPI(c *fi.Context, cluster *kops.Cluster, ig *kops.InstanceGroup, tasks map[string]fi.Task) ([]*unstructured.Unstructured, error)
}

func (b *Builder) BuildMachineDeployment(ctx context.Context, clusterObj *kopsv1alpha2.Cluster, igObj *kopsv1alpha2.InstanceGroup) ([]*unstructured.Unstructured, error) {
	cloudup.AlphaAllowGCE.SetEnabled(true)

	cluster := &kops.Cluster{}
	{
		copy := clusterObj.DeepCopy()
		copy.Namespace = ""

		if err := kopscodecs.Scheme.Convert(copy, cluster, nil); err != nil {
			return nil, fmt.Errorf("error converting cluster to internal form: %v", err)
		}
	}

	ig := &kops.InstanceGroup{}
	{
		if err := kopscodecs.Scheme.Convert(igObj, ig, nil); err != nil {
			return nil, fmt.Errorf("error converting InstanceGroup to internal form: %v", err)
		}
	}

	phase := cloudup.PhaseCluster

	clusterConfigBase, err := registry.ConfigBase(cluster)
	if err != nil {
		return nil, err
	}

	var keyStore fi.CAStore
	{
		basedir := clusterConfigBase.Join("pki")
		keyStore = fi.NewVFSCAStore(cluster, basedir)
	}

	var secretStore fi.SecretStore

	{
		basedir := clusterConfigBase.Join("secrets")
		secretStore = secrets.NewVFSSecretStore(cluster, basedir)
	}

	var sshCredentialStore fi.SSHCredentialStore
	{
		configBase, err := registry.ConfigBase(cluster)
		if err != nil {
			return nil, fmt.Errorf("unable to get config base: %w", err)
		}
		basedir := configBase.Join("pki")
		sshCredentialStore = fi.NewVFSSSHCredentialStore(cluster, basedir)
	}

	cloud, err := cloudup.BuildCloud(cluster)
	if err != nil {
		return nil, err
	}

	{
		assetBuilder := assets.NewAssetBuilder(cluster, string(phase))

		fullCluster, err := cloudup.PopulateClusterSpec(cluster, cloud, assetBuilder, keyStore, secretStore)
		if err != nil {
			return nil, err
		}
		cluster = fullCluster

		// The instance group is populated in place; no need to hydrate
		/*
			fullGroup, err := cloudup.PopulateInstanceGroupSpec(fullCluster, g, c.channel)
			if err != nil {
				return nil, err
			}
			ig = fullGroup
		*/
	}

	// applyCmd := &cloudup.ApplyClusterCmd{
	// 	Cluster:        cluster,
	// 	Clientset:      b.Clientset,
	// 	InstanceGroups: []*kops.InstanceGroup{ig},
	// 	Phase:          phase,
	// }

	instanceGroups := []*kops.InstanceGroup{ig}
	targetName := cloudup.TargetDirect
	var addons kubemanifest.ObjectList

	loader, assetBuilder, err := cloudup.BuildLoader(ctx, cluster, instanceGroups, cloud, keyStore, secretStore, sshCredentialStore, phase, targetName, addons)
	if err != nil {
		return nil, err
	}

	var objects []*unstructured.Unstructured

	stageAssetsLifecycle := fi.LifecycleIgnore

	predicate := func(builder fi.ModelBuilder) bool {
		_, ok := builder.(ClusterAPIBuilder)
		return ok
	}
	tasks, err := loader.BuildTasks(assetBuilder, &stageAssetsLifecycle, nil, predicate)
	if err != nil {
		return nil, fmt.Errorf("error building tasks: %w", err)
	}

	// TODO: Maybe getting the loaders is better!

	// stageAssetsLifecycle := fi.LifecycleIgnore
	// switch phase {
	// case PhaseStageAssets:
	// 	stageAssetsLifecycle = fi.LifecycleSync
	// }

	// taskMap, err := loader.BuildTasks(assetBuilder, &stageAssetsLifecycle, lifecycleOverrides)
	// if err != nil {
	// 	return nil, fmt.Errorf("error building tasks: %v", err)
	// }

	// return taskMap, nil

	var mappers []ClusterAPIBuilder

	// tasks := make(map[string]fi.Task)
	for _, builder := range loader.Builders {
		clusterAPIBuilder, ok := builder.(ClusterAPIBuilder)
		if !ok {
			continue
		}
		mappers = append(mappers, clusterAPIBuilder)

		// context := &fi.ModelBuilderContext{
		// 	Tasks:              tasks,
		// 	LifecycleOverrides: nil,
		// }
		// err := builder.Build(context)
		// if err != nil {
		// 	return nil, err
		// }
		// tasks = context.Tasks

		// clusterAPIBuilder, ok := b.(ClusterAPIBuilder)
		// if !ok {
		// 	continue
		// }

		// objs, err := clusterAPIBuilder.MapToClusterAPI(cluster, ig)
		// if err != nil {
		// 	return nil, err
		// }

		// objects = append(objects, objs...)
	}

	// var mappers []func(cluster *kops.Cluster, ig *kops.InstanceGroup, tasks map[string]fi.Task) ([]*unstructured.Unstructured, error)
	// mappers = append(mappers, gcemodel.MapToClusterAPI)

	var out bytes.Buffer
	target := fi.NewDryRunTarget(assetBuilder, &out)
	checkExisting := true
	c, err := fi.NewContext(target, cluster, cloud, keyStore, secretStore, clusterConfigBase, checkExisting, tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to build execution context: %w", err)
	}

	var options fi.RunTasksOptions
	options.InitDefaults()

	if err := c.RunTasks(options); err != nil {
		return nil, fmt.Errorf("error running tasks: %w", err)
	}

	// for _, task := range tasks {
	// 	if err := task.Run(c); err != nil {
	// 		return nil, fmt.Errorf("error running task: %w", err)
	// 	}
	// }

	for _, mapper := range mappers {
		objs, err := mapper.MapToClusterAPI(c, cluster, ig, tasks)
		if err != nil {
			return nil, err
		}

		objects = append(objects, objs...)
	}

	for _, obj := range objects {
		owners := obj.GetOwnerReferences()
		if len(owners) != 0 {
			// We could implement this, but for now we guard against it
			return nil, fmt.Errorf("existing owner refs not yet supported")
		}

		blockOwnerDeletion := true
		controller := true
		owners = append(owners, metav1.OwnerReference{
			Name:               ig.Name,
			Kind:               "InstanceGroup",
			APIVersion:         kopsv1alpha2.SchemeGroupVersion.String(),
			UID:                ig.UID,
			BlockOwnerDeletion: &blockOwnerDeletion,
			Controller:         &controller,
		})

		obj.SetOwnerReferences(owners)
	}

	for _, obj := range objects {
		klog.Infof("object: %v", fi.DebugAsJsonString(obj))
	}

	return objects, nil
}
