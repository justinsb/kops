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

package clusteraddons

import (
	"testing"

	"k8s.io/kops/pkg/kubemanifest"
)

func TestMerge(t *testing.T) {
	load := func(y string) kubemanifest.ObjectList {
		objects, err := kubemanifest.LoadObjectsFrom([]byte(y))
		if err != nil {
			t.Fatal(err)
		}
		return objects
	}
	existing := load(`
apiVersion: v1
kind: ConfigMap
metadata: {name: a, namespace: default}
data: {v: "1"}
---
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata: {name: cert-manager}
spec: {version: v1}
`)
	updates := load(`
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata: {name: cert-manager}
spec: {version: v2}
---
apiVersion: v1
kind: ConfigMap
metadata: {name: a, namespace: other}
data: {v: "2"}
`)
	merged := Merge(existing, updates)
	if len(merged) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(merged))
	}
	// Position of the replaced object is kept, and its content updated.
	u := merged[1].ToUnstructured()
	if u.GetKind() != "ClusterPackage" || u.Object["spec"].(map[string]interface{})["version"] != "v2" {
		t.Errorf("ClusterPackage was not replaced in place: %v", u.Object)
	}
	if merged[2].ToUnstructured().GetNamespace() != "other" {
		t.Errorf("new object was not appended")
	}
	// Merging with no existing objects returns the updates.
	if got := Merge(nil, updates); len(got) != 2 {
		t.Errorf("Merge(nil, updates) = %d objects", len(got))
	}
}
