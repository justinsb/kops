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

package metal

import (
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/upup/pkg/fi"
)

// IGConfigBuilder writes igconfig/nodeupconfig.yaml files into the cluster state
// store for each instance group. Cloud providers do this indirectly via their
// instance builders; bare metal has no cloud instances, so we write the files
// directly during kops update cluster.
type IGConfigBuilder struct {
	*model.KopsModelContext
	Lifecycle              fi.Lifecycle
	BootstrapScriptBuilder *model.BootstrapScriptBuilder
}

var _ fi.CloudupModelBuilder = &IGConfigBuilder{}

func (b *IGConfigBuilder) Build(c *fi.CloudupModelBuilderContext) error {
	for _, ig := range b.InstanceGroups {
		if _, err := b.BootstrapScriptBuilder.ResourceNodeUp(c, ig); err != nil {
			return err
		}
	}
	return nil
}
