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
	"encoding/hex"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// allowedManifestSchemes are the URL schemes a ClusterPackage manifest may be fetched from.
// Plain http is deliberately excluded.
var allowedManifestSchemes = map[string]bool{"https": true, "gs": true, "s3": true, "file": true, "memfs": true}

// ValidateObject validates a ClusterPackage or GCPResource. Objects in other groups are ignored.
func ValidateObject(fieldPath *field.Path, u *unstructured.Unstructured) field.ErrorList {
	gvk := u.GroupVersionKind()
	if gvk.Group != GroupName {
		return nil
	}
	var errs field.ErrorList
	if gvk.Version != Version {
		errs = append(errs, field.NotSupported(fieldPath.Child("apiVersion"), gvk.GroupVersion().String(), []string{GroupName + "/" + Version}))
		return errs
	}
	switch gvk.Kind {
	case KindClusterPackage:
		if name := u.GetName(); name == "" {
			errs = append(errs, field.Required(fieldPath.Child("metadata", "name"), ""))
		} else if msgs := validation.IsValidLabelValue(name); len(msgs) != 0 {
			// The name becomes the channel addon name and the "k8s-addon" selector label value.
			errs = append(errs, field.Invalid(fieldPath.Child("metadata", "name"), name, "must be a valid label value: "+strings.Join(msgs, "; ")))
		}
		if u.GetNamespace() != "" {
			errs = append(errs, field.Forbidden(fieldPath.Child("metadata", "namespace"), "ClusterPackage is cluster-scoped"))
		}
		spec, err := ParseClusterPackageSpec(u)
		if err != nil {
			errs = append(errs, field.Invalid(fieldPath.Child("spec"), nil, err.Error()))
			return errs
		}
		for _, e := range validateClusterPackageSpec(spec) {
			e.Field = fieldPath.Child("spec").String() + "." + e.Field
			errs = append(errs, e)
		}
	case KindGCPResource:
		name := u.GetName()
		if name == "" {
			errs = append(errs, field.Required(fieldPath.Child("metadata", "name"), "the Google Cloud resource URL"))
		} else if _, _, err := ParseResourceURL(name); err != nil {
			// The name may contain template actions; only validate the shape when it does not.
			if !containsTemplate(name) {
				errs = append(errs, field.Invalid(fieldPath.Child("metadata", "name"), name, err.Error()))
			}
		}
		if _, found, err := unstructured.NestedMap(u.Object, "spec"); err != nil {
			errs = append(errs, field.Invalid(fieldPath.Child("spec"), nil, "spec must be an object"))
		} else if !found {
			errs = append(errs, field.Required(fieldPath.Child("spec"), "the resource body"))
		}
	default:
		errs = append(errs, field.NotSupported(fieldPath.Child("kind"), gvk.Kind, []string{KindClusterPackage, KindGCPResource}))
	}
	return errs
}

func validateClusterPackageSpec(spec *ClusterPackageSpec) field.ErrorList {
	var errs field.ErrorList
	if spec.Version == "" {
		errs = append(errs, field.Required(field.NewPath("version"), "the package version"))
	}
	m := spec.Manifest
	switch {
	case m.URL != "" && m.Inline != "":
		errs = append(errs, field.Invalid(field.NewPath("manifest"), "", "only one of url and inline may be set"))
	case m.URL == "" && m.Inline == "":
		errs = append(errs, field.Required(field.NewPath("manifest"), "one of url or inline"))
	case m.URL != "":
		u, err := url.Parse(m.URL)
		if err != nil || !u.IsAbs() {
			errs = append(errs, field.Invalid(field.NewPath("manifest", "url"), m.URL, "must be an absolute URL"))
		} else if !allowedManifestSchemes[u.Scheme] {
			errs = append(errs, field.NotSupported(field.NewPath("manifest", "url"), u.Scheme, []string{"https", "gs", "s3", "file"}))
		}
	}
	if m.SHA256 != "" {
		if b, err := hex.DecodeString(m.SHA256); err != nil || len(b) != 32 {
			errs = append(errs, field.Invalid(field.NewPath("manifest", "sha256"), m.SHA256, "must be 64 hex characters"))
		}
		if m.Inline != "" {
			errs = append(errs, field.Invalid(field.NewPath("manifest", "sha256"), m.SHA256, "sha256 only applies to url"))
		}
	}
	return errs
}

func containsTemplate(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '{' && s[i+1] == '{' {
			return true
		}
	}
	return false
}
