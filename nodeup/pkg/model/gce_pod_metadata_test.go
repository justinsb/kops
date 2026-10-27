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

package model

import (
	"strings"
	"testing"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/nodeup"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/nodetasks"
)

func TestGCEPodMetadataBuilder(t *testing.T) {
	tests := []struct {
		name      string
		cloud     kops.CloudProviderID
		enabled   bool
		wantTasks bool
	}{
		{name: "enabled on GCE", cloud: kops.CloudProviderGCE, enabled: true, wantTasks: true},
		{name: "disabled on GCE", cloud: kops.CloudProviderGCE, enabled: false, wantTasks: false},
		{name: "flag set but not GCE", cloud: kops.CloudProviderAWS, enabled: true, wantTasks: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &GCEPodMetadataBuilder{NodeupModelContext: &NodeupModelContext{
				BootConfig:   &nodeup.BootConfig{CloudProvider: tc.cloud},
				NodeupConfig: &nodeup.Config{GCEBlockPodMetadataAccess: tc.enabled},
			}}
			c := &fi.NodeupModelBuilderContext{Tasks: map[string]fi.NodeupTask{}}
			if err := b.Build(c); err != nil {
				t.Fatal(err)
			}
			if !tc.wantTasks {
				if len(c.Tasks) != 0 {
					t.Fatalf("expected no tasks, got %v", c.Tasks)
				}
				return
			}
			var script *nodetasks.File
			var service *nodetasks.Service
			for _, task := range c.Tasks {
				switch task := task.(type) {
				case *nodetasks.File:
					script = task
				case *nodetasks.Service:
					service = task
				}
			}
			if script == nil || service == nil {
				t.Fatalf("expected a script and a service, got %v", c.Tasks)
			}
			body, err := fi.ResourceAsString(script.Contents)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"ip addr replace 169.254.169.252/32 dev lo",
				"-d 169.254.169.254",
				"-j DNAT --to-destination 169.254.169.252",
				"iptables -w -t nat -C PREROUTING",
			} {
				if !strings.Contains(body, want) {
					t.Errorf("script is missing %q:\n%s", want, body)
				}
			}
			if !strings.Contains(*service.Definition, "Before=network.target") {
				t.Errorf("service should run before network.target:\n%s", *service.Definition)
			}
		})
	}
}
