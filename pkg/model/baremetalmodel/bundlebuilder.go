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

package baremetalmodel

import (
	"github.com/golang/glog"
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/fitasks"
)

// BundleModelBuilder builds the bundle
type BundleModelBuilder struct {
	*BaremetalModelContext
	Lifecycle *fi.Lifecycle

	BootstrapScript *model.BootstrapScript
}

var _ fi.ModelBuilder = &BundleModelBuilder{}

func (b *BundleModelBuilder) Build(c *fi.ModelBuilderContext) error {
	glog.Warningf("TODO: Harmonize gcemodel with awsmodel for firewall - GCE model is way too open")

	for _, ig := range b.InstanceGroups {
		kubeEnv, err := b.BootstrapScript.KubeEnv(ig)
		if err != nil {
			return err
		}

		location := "nodeup/ig/" + ig.Name

		c.AddTask(&fitasks.ManagedFile{
			Name:      fi.String("kubeenv-" + ig.Name),
			Lifecycle: b.Lifecycle,
			Location:  fi.String(location),
			Contents:  fi.WrapResource(fi.NewStringResource(kubeEnv)),
		})
	}

	return nil
}
