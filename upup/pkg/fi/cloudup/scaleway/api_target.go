/*
Copyright 2022 The Kubernetes Authors.

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

package scaleway

import "k8s.io/kops/upup/pkg/fi"

type ScwAPITarget struct {
	Cloud ScwCloud
}

var _ fi.CloudupTarget = &ScwAPITarget{}

func NewScwAPITarget(cloud ScwCloud) *ScwAPITarget {
	return &ScwAPITarget{
		Cloud: cloud,
	}
}

func (s ScwAPITarget) Finish(taskMap map[string]fi.CloudupTask) error {
	return nil
}

func (t *ScwAPITarget) DefaultCheckExisting() bool {
	return true
}
