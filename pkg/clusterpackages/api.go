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

// Package clusterpackages implements the ClusterPackage and GCPResource addon objects
// (group addons.kops.k8s.io). A ClusterPackage references a Kubernetes manifest that kOps
// installs through the channels mechanism; a GCPResource declares a Google Cloud resource
// that the package needs, such as an enabled API or an IAM binding.
package clusterpackages

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the API group of the addon objects handled by this package.
	GroupName = "addons.kops.k8s.io"
	// Version is the only API version currently supported.
	Version = "v1alpha1"
	// KindClusterPackage is the kind of a ClusterPackage object.
	KindClusterPackage = "ClusterPackage"
	// KindGCPResource is the kind of a GCPResource object.
	KindGCPResource = "GCPResource"
)

// ClusterPackageGVK is the GroupVersionKind of ClusterPackage.
var ClusterPackageGVK = schema.GroupVersionKind{Group: GroupName, Version: Version, Kind: KindClusterPackage}

// GCPResourceGVK is the GroupVersionKind of GCPResource.
var GCPResourceGVK = schema.GroupVersionKind{Group: GroupName, Version: Version, Kind: KindGCPResource}

// ClusterPackageSpec is the spec of a ClusterPackage object.
type ClusterPackageSpec struct {
	// Version identifies the package version. It becomes the channel addon id, so changing it
	// causes the manifest to be re-applied.
	Version string `json:"version"`
	// Manifest is where the Kubernetes manifest comes from.
	Manifest ManifestSource `json:"manifest"`
	// Render, when true, treats the manifest as a Go template rendered against Values.
	// Defaults to false: the manifest is applied verbatim.
	Render *bool `json:"render,omitempty"`
	// Prune, when true (the default), deletes objects that this package previously created
	// but that are no longer in the manifest.
	Prune *bool `json:"prune,omitempty"`
}

// ManifestSource locates a manifest. Exactly one of URL or Inline must be set.
type ManifestSource struct {
	// URL is a VFS URL (https://, gs://, s3://, file://) of the manifest.
	URL string `json:"url,omitempty"`
	// SHA256 optionally pins the hex SHA-256 of the manifest fetched from URL.
	SHA256 string `json:"sha256,omitempty"`
	// Inline is the manifest content itself, for small packages and tests.
	Inline string `json:"inline,omitempty"`
}

// ParseClusterPackageSpec extracts the spec from a ClusterPackage object.
func ParseClusterPackageSpec(u *unstructured.Unstructured) (*ClusterPackageSpec, error) {
	specObj, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("spec is required")
	}
	spec := &ClusterPackageSpec{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(specObj, spec); err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}
	return spec, nil
}

// IsClusterPackage reports whether the object is a ClusterPackage in our group.
func IsClusterPackage(u *unstructured.Unstructured) bool {
	gvk := u.GroupVersionKind()
	return gvk.Group == GroupName && gvk.Kind == KindClusterPackage
}

// IsGCPResource reports whether the object is a GCPResource in our group.
func IsGCPResource(u *unstructured.Unstructured) bool {
	gvk := u.GroupVersionKind()
	return gvk.Group == GroupName && gvk.Kind == KindGCPResource
}
