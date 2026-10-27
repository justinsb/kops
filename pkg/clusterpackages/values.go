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
	"bytes"
	"fmt"
	"text/template"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
)

// Values is the deliberately small set of cluster facts available to rendered manifests and to
// GCPResource names and specs. Keeping it small keeps it a stable API; the internal addon template
// functions are not exposed to packages.
type Values struct {
	// ClusterName is the kOps cluster name, e.g. "gw.k8s.local".
	ClusterName string
	// CloudProvider is the cloud provider id, e.g. "gce" or "aws".
	CloudProvider string
	// Region is the cloud region of the cluster's subnets, when the cloud provider models one.
	Region string
	// KubernetesVersion is the cluster's kubernetes version, e.g. "1.37.1".
	KubernetesVersion string
	// GCP is set when the cluster runs on Google Cloud.
	GCP *GCPValues
}

// GCPValues are the Google Cloud facts available to packages.
type GCPValues struct {
	// Project is the GCP project id.
	Project string
	// ControlPlaneServiceAccount is the email of the service account the control plane runs as.
	ControlPlaneServiceAccount string
	// NodeServiceAccount is the email of the service account worker nodes run as.
	NodeServiceAccount string
}

// ValuesForCluster builds the Values for a cluster.
func ValuesForCluster(cluster *kops.Cluster) Values {
	v := Values{
		ClusterName:       cluster.Name,
		CloudProvider:     string(cluster.GetCloudProvider()),
		KubernetesVersion: cluster.Spec.KubernetesVersion,
	}
	for _, subnet := range cluster.Spec.Networking.Subnets {
		if subnet.Region != "" {
			v.Region = subnet.Region
			break
		}
	}
	if cluster.Spec.CloudProvider.GCE != nil {
		project := cluster.Spec.CloudProvider.GCE.Project
		v.GCP = &GCPValues{
			Project:                    project,
			ControlPlaneServiceAccount: gce.ServiceAccountName(gce.ControlPlane, cluster.Name) + "@" + project + ".iam.gserviceaccount.com",
			NodeServiceAccount:         gce.ServiceAccountName(gce.Node, cluster.Name) + "@" + project + ".iam.gserviceaccount.com",
		}
	}
	return v
}

// Render renders text as a Go template against values. Unknown fields are errors rather than
// empty strings, so a typo in a package fails at update time instead of producing a broken manifest.
func Render(name string, text string, values Values) (string, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("rendering template %q: %w", name, err)
	}
	return buf.String(), nil
}

// RenderStrings renders every string found in a JSON-like value (maps, slices, strings) in place
// semantics-preserving: structure is kept, only strings containing a template action are rendered.
func RenderStrings(name string, v interface{}, values Values) (interface{}, error) {
	switch t := v.(type) {
	case string:
		if !bytes.Contains([]byte(t), []byte("{{")) {
			return t, nil
		}
		return Render(name, t, values)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, child := range t {
			rendered, err := RenderStrings(name+"."+k, child, values)
			if err != nil {
				return nil, err
			}
			out[k] = rendered
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, child := range t {
			rendered, err := RenderStrings(fmt.Sprintf("%s[%d]", name, i), child, values)
			if err != nil {
				return nil, err
			}
			out[i] = rendered
		}
		return out, nil
	default:
		return v, nil
	}
}
