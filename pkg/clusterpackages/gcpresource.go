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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GCPResource is a parsed GCPResource object. The object's name is the Google Cloud resource URL,
// in the "//service.googleapis.com/path" form used by Cloud Asset Inventory, and the spec is the
// resource body in that API's own schema (a subset of what GET returns).
type GCPResource struct {
	// Name is the rendered resource URL, e.g. "//serviceusage.googleapis.com/projects/p/services/dns.googleapis.com".
	Name string
	// Service is the API host, e.g. "serviceusage.googleapis.com".
	Service string
	// Path is the resource path within the service, e.g. "projects/p/services/dns.googleapis.com".
	Path string
	// Spec is the rendered resource body.
	Spec map[string]interface{}
}

// ParseGCPResource parses and renders a GCPResource object.
func ParseGCPResource(values Values, u *unstructured.Unstructured) (*GCPResource, error) {
	rawName := u.GetName()
	if rawName == "" {
		return nil, fmt.Errorf("GCPResource has no metadata.name")
	}
	name, err := Render("GCPResource "+rawName, rawName, values)
	if err != nil {
		return nil, err
	}
	service, path, err := ParseResourceURL(name)
	if err != nil {
		return nil, fmt.Errorf("GCPResource %q: %w", rawName, err)
	}

	spec, found, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("GCPResource %q: reading spec: %w", rawName, err)
	}
	if !found {
		spec = map[string]interface{}{}
	}
	rendered, err := RenderStrings("GCPResource "+rawName+" spec", spec, values)
	if err != nil {
		return nil, fmt.Errorf("GCPResource %q: %w", rawName, err)
	}
	return &GCPResource{Name: name, Service: service, Path: path, Spec: rendered.(map[string]interface{})}, nil
}

// ParseResourceURL splits a "//service/path" resource URL into its service host and path.
func ParseResourceURL(name string) (service string, path string, err error) {
	if !strings.HasPrefix(name, "//") {
		return "", "", fmt.Errorf("resource URL %q must start with //", name)
	}
	rest := strings.TrimPrefix(name, "//")
	i := strings.Index(rest, "/")
	if i <= 0 || i == len(rest)-1 {
		return "", "", fmt.Errorf("resource URL %q must have the form //service.googleapis.com/path", name)
	}
	service, path = rest[:i], rest[i+1:]
	if !strings.HasSuffix(service, ".googleapis.com") {
		return "", "", fmt.Errorf("resource URL %q: service %q must be a *.googleapis.com host", name, service)
	}
	return service, path, nil
}

// PathSegments splits a resource path into key/value pairs: "projects/p/services/s" ->
// {"projects": "p", "services": "s"}. It is a convenience for the per-service adapters.
func (r *GCPResource) PathSegments() (map[string]string, error) {
	parts := strings.Split(r.Path, "/")
	if len(parts)%2 != 0 {
		return nil, fmt.Errorf("resource path %q does not consist of collection/name pairs", r.Path)
	}
	out := make(map[string]string, len(parts)/2)
	for i := 0; i < len(parts); i += 2 {
		if parts[i] == "" || parts[i+1] == "" {
			return nil, fmt.Errorf("resource path %q has an empty segment", r.Path)
		}
		out[parts[i]] = parts[i+1]
	}
	return out, nil
}
