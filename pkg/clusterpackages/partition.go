/*
Copyright 2026 The Kubernetes Authors.

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

package clusterpackages

import "k8s.io/kops/pkg/kubemanifest"

// Partition groups addon objects by how kOps handles them.
type Partition struct {
	// ClusterPackages are the ClusterPackage objects.
	ClusterPackages kubemanifest.ObjectList
	// GCPResources are the GCPResource objects.
	GCPResources kubemanifest.ObjectList
	// Other is everything else, which is applied to the cluster as-is.
	Other kubemanifest.ObjectList
}

// PartitionObjects splits addon objects into ClusterPackages, GCPResources and the rest.
func PartitionObjects(objects kubemanifest.ObjectList) Partition {
	var p Partition
	for _, object := range objects {
		u := object.ToUnstructured()
		switch {
		case IsClusterPackage(u):
			p.ClusterPackages = append(p.ClusterPackages, object)
		case IsGCPResource(u):
			p.GCPResources = append(p.GCPResources, object)
		default:
			p.Other = append(p.Other, object)
		}
	}
	return p
}
