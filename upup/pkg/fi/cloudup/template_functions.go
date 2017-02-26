/*
Copyright 2016 The Kubernetes Authors.

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

/******************************************************************************
Template Functions are what map functions in the models, to internal logic in
kops. This is the point where we connect static YAML configuration to dynamic
runtime values in memory.

When defining a new function:
	- Build the new function here
	- Define the new function in AddTo()
		dest["MyNewFunction"] = MyNewFunction // <-- Function Pointer
******************************************************************************/

package cloudup

import (
	"encoding/base64"
	"fmt"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/model/components"
	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
	"k8s.io/kubernetes/pkg/util/sets"
	"strings"
	"text/template"
)

type TemplateFunctions struct {
	cluster        *api.Cluster
	instanceGroups []*api.InstanceGroup

	tags   sets.String
	region string

	modelContext *model.KopsModelContext
}

// This will define the available functions we can use in our YAML models
// If we are trying to get a new function implemented it MUST
// be defined here.
func (tf *TemplateFunctions) AddTo(dest template.FuncMap) {
	dest["SharedVPC"] = tf.SharedVPC

	// Remember that we may be on a different arch from the target.  Hard-code for now.
	dest["Arch"] = func() string { return "amd64" }

	// Network topology definitions
	dest["GetELBName32"] = tf.modelContext.GetELBName32

	dest["Base64Encode"] = func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	dest["replace"] = func(s, find, replace string) string {
		return strings.Replace(s, find, replace, -1)
	}
	dest["join"] = func(a []string, sep string) string {
		return strings.Join(a, sep)
	}

	dest["ClusterName"] = tf.modelContext.ClusterName

	dest["HasTag"] = tf.HasTag

	dest["Image"] = tf.Image

	dest["WithDefaultBool"] = func(v *bool, defaultValue bool) bool {
		if v != nil {
			return *v
		}
		return defaultValue
	}

	dest["GetInstanceGroup"] = tf.GetInstanceGroup

	dest["CloudTags"] = tf.modelContext.CloudTagsForInstanceGroup

	dest["KubeDNS"] = func() *api.KubeDNSConfig {
		return tf.cluster.Spec.KubeDNS
	}

	// TODO: Only for GCE?
	dest["EncodeGCELabel"] = gce.EncodeGCELabel

}

// SharedVPC is a simple helper function which makes the templates for a shared VPC clearer
func (tf *TemplateFunctions) SharedVPC() bool {
	return tf.cluster.SharedVPC()
}

// Image returns the docker image name for the specified component
func (tf *TemplateFunctions) Image(component string) (string, error) {
	return components.Image(component, &tf.cluster.Spec)
}

// HasTag returns true if the specified tag is set
func (tf *TemplateFunctions) HasTag(tag string) bool {
	_, found := tf.tags[tag]
	return found
}

// GetInstanceGroup returns the instance group with the specified name
func (tf *TemplateFunctions) GetInstanceGroup(name string) (*api.InstanceGroup, error) {
	for _, ig := range tf.instanceGroups {
		if ig.ObjectMeta.Name == name {
			return ig, nil
		}
	}
	return nil, fmt.Errorf("InstanceGroup %q not found", name)
}
