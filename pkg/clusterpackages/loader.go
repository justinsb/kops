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

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/util/pkg/vfs"
)

// Package is a loaded ClusterPackage: the manifest bytes ready to hand to the channels mechanism.
type Package struct {
	// Name is the ClusterPackage name; it is also the channel addon name.
	Name string
	// Version is spec.version; it is used as the channel addon id.
	Version string
	// Manifest is the (possibly rendered) manifest.
	Manifest []byte
	// Prune is whether objects removed from the manifest should be deleted from the cluster.
	Prune bool
}

// LoadPackage fetches and prepares the manifest for a ClusterPackage object.
func LoadPackage(ctx context.Context, vfsContext *vfs.VFSContext, values Values, u *unstructured.Unstructured) (*Package, error) {
	name := u.GetName()
	if name == "" {
		return nil, fmt.Errorf("ClusterPackage has no metadata.name")
	}
	spec, err := ParseClusterPackageSpec(u)
	if err != nil {
		return nil, fmt.Errorf("ClusterPackage %q: %w", name, err)
	}
	if errs := validateClusterPackageSpec(spec); len(errs) != 0 {
		return nil, fmt.Errorf("ClusterPackage %q: %w", name, errs.ToAggregate())
	}

	var manifest []byte
	switch {
	case spec.Manifest.Inline != "":
		manifest = []byte(spec.Manifest.Inline)
	default:
		manifest, err = vfsContext.ReadFile(spec.Manifest.URL)
		if err != nil {
			return nil, fmt.Errorf("ClusterPackage %q: reading manifest %q: %w", name, spec.Manifest.URL, err)
		}
		if spec.Manifest.SHA256 != "" {
			sum := sha256.Sum256(manifest)
			actual := hex.EncodeToString(sum[:])
			if !strings.EqualFold(actual, spec.Manifest.SHA256) {
				return nil, fmt.Errorf("ClusterPackage %q: manifest %q has sha256 %s, expected %s", name, spec.Manifest.URL, actual, spec.Manifest.SHA256)
			}
		}
	}

	if spec.Render != nil && *spec.Render {
		rendered, err := Render(name, string(manifest), values)
		if err != nil {
			return nil, fmt.Errorf("ClusterPackage %q: %w", name, err)
		}
		manifest = []byte(rendered)
	}

	// Parse to catch malformed manifests here, with the package name in the error,
	// rather than inside the channels agent on the control plane.
	if _, err := kubemanifest.LoadObjectsFrom(manifest); err != nil {
		return nil, fmt.Errorf("ClusterPackage %q: manifest is not valid YAML/JSON: %w", name, err)
	}

	prune := true
	if spec.Prune != nil {
		prune = *spec.Prune
	}
	return &Package{Name: name, Version: spec.Version, Manifest: manifest, Prune: prune}, nil
}
