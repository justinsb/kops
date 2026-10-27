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
	"k8s.io/kops/pkg/kubemanifest"
)

// Merge returns existing with updates applied: an update replaces an existing object with the
// same group, kind, namespace and name (keeping its position), and is appended otherwise.
func Merge(existing, updates kubemanifest.ObjectList) kubemanifest.ObjectList {
	type key struct{ group, kind, namespace, name string }
	keyOf := func(o *kubemanifest.Object) key {
		u := o.ToUnstructured()
		gvk := u.GroupVersionKind()
		return key{gvk.Group, gvk.Kind, u.GetNamespace(), u.GetName()}
	}

	index := make(map[key]int, len(existing))
	merged := make(kubemanifest.ObjectList, 0, len(existing)+len(updates))
	for _, o := range existing {
		index[keyOf(o)] = len(merged)
		merged = append(merged, o)
	}
	for _, o := range updates {
		k := keyOf(o)
		if i, ok := index[k]; ok {
			merged[i] = o
			continue
		}
		index[k] = len(merged)
		merged = append(merged, o)
	}
	return merged
}
