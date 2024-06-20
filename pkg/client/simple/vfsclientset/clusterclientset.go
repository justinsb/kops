// /*
// Copyright 2024 The Kubernetes Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package vfsclientset

// import (
// 	"context"

// 	"k8s.io/klog/v2"

// 	"k8s.io/kops/pkg/apis/kops"
// 	"k8s.io/kops/pkg/apis/kops/registry"
// 	kopsinternalversion "k8s.io/kops/pkg/client/clientset_generated/clientset/typed/kops/internalversion"
// 	"k8s.io/kops/pkg/client/simple"
// 	"k8s.io/kops/upup/pkg/fi"
// 	"k8s.io/kops/upup/pkg/fi/secrets"
// 	"k8s.io/kops/util/pkg/vfs"
// )

// type VFSClusterClientset struct {
// 	clientset   simple.Clientset
// 	clusterName string
// }

// func NewVFSClusterClientset(vfsContext *vfs.VFSContext, clusterBasePath vfs.Path) simple.ClusterClientset {
// 	parent := clusterBasePath.Dir()
// 	clusterName := clusterBasePath.Base()
// 	clientset := NewVFSClientset(vfsContext, parent)
// 	ret := &VFSClusterClientset{
// 		clientset:   clientset,
// 		clusterName: clusterName,
// 	}
// 	return ret
// }

// var _ simple.ClusterClientset = &VFSClusterClientset{}

// func (c *VFSClusterClientset) VFSContext() *vfs.VFSContext {
// 	return c.vfsContext
// }

// func (c *VFSClusterClientset) clusters() *ClusterVFS {
// 	return newClusterVFS(c.VFSContext(), c.basePath)
// }

// // GetCluster implements the GetCluster method of simple.Clientset for a VFS-backed state store
// func (c *VFSClusterClientset) GetCluster(ctx context.Context) (*kops.Cluster, error) {
// 	return c.clientset.GetCluster(ctx, c.clusterName)
// }

// // ConfigBaseFor implements the ConfigBase method of simple.ClusterClientset for a VFS-backed state store
// func (c *VFSClusterClientset) ConfigBase() (vfs.Path, error) {
// 	if cluster.Spec.ConfigStore.Base != "" {
// 		return c.VFSContext().BuildVfsPath(cluster.Spec.ConfigStore.Base)
// 	}
// 	return c.clusters().configBase(cluster.Name)
// }

// // InstanceGroupsFor implements the InstanceGroups method of simple.Clientset for a VFS-backed state store
// func (c *VFSClusterClientset) InstanceGroups() kopsinternalversion.InstanceGroupInterface {
// 	return newInstanceGroupVFS(c, cluster)
// }

// func (c *VFSClusterClientset) Addons() simple.AddonsClient {
// 	return newAddonsVFS(c, cluster)
// }

// func (c *VFSClusterClientset) SecretStore() (fi.SecretStore, error) {
// 	if cluster.Spec.ConfigStore.Secrets == "" {
// 		configBase, err := registry.ConfigBase(c.VFSContext(), cluster)
// 		if err != nil {
// 			return nil, err
// 		}
// 		basedir := configBase.Join("secrets")
// 		return secrets.NewVFSSecretStore(cluster, basedir), nil
// 	} else {
// 		storePath, err := c.VFSContext().BuildVfsPath(cluster.Spec.ConfigStore.Secrets)
// 		return secrets.NewVFSSecretStore(cluster, storePath), err
// 	}
// }

// func (c *VFSClusterClientset) KeyStore() (fi.CAStore, error) {
// 	basedir, err := c.pkiPath(cluster)
// 	if err != nil {
// 		return nil, err
// 	}

// 	klog.Infof("Using keystore path: %q", basedir)

// 	return fi.NewVFSCAStore(cluster, basedir), err
// }

// func (c *VFSClusterClientset) SSHCredentialStore() (fi.SSHCredentialStore, error) {
// 	basedir, err := c.pkiPath(cluster)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return fi.NewVFSSSHCredentialStore(cluster, basedir), nil
// }

// func (c *VFSClusterClientset) pkiPath(cluster *kops.Cluster) (vfs.Path, error) {
// 	if cluster.Spec.ConfigStore.Keypairs == "" {
// 		configBase, err := registry.ConfigBase(c.VFSContext(), cluster)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return configBase.Join("pki"), nil
// 	} else {
// 		storePath, err := c.VFSContext().BuildVfsPath(cluster.Spec.ConfigStore.Keypairs)
// 		return storePath, err
// 	}
// }
