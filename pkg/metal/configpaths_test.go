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
	"testing"

	"k8s.io/kops/pkg/apis/kops"
)

func TestLocalConfigFileURL(t *testing.T) {
	got := LocalConfigFileURL()
	want := "file:///etc/kubernetes/kops/config"
	if got != want {
		t.Fatalf("LocalConfigFileURL = %q, want %q", got, want)
	}
}

func TestKopsControllerStorePaths(t *testing.T) {
	cluster := &kops.Cluster{}
	cluster.Spec.ConfigStore.Base = "s3://kops-state-721322707521-justinsb/homebase1"

	configBase, secretStore := KopsControllerStorePaths(cluster)
	if configBase != "/etc/kubernetes/kops/config" {
		t.Fatalf("configBase = %q", configBase)
	}
	if secretStore != "/etc/kubernetes/kops/config/pki" {
		t.Fatalf("secretStore = %q", secretStore)
	}
}

func TestMirrorClusterRelativePath(t *testing.T) {
	tests := []struct {
		relative string
		want     string
		ok       bool
	}{
		{relative: "igconfig/node/nodes-main/nodeupconfig.yaml", want: "/etc/kubernetes/kops/config/igconfig/node/nodes-main/nodeupconfig.yaml", ok: true},
		{relative: "secrets/kubelet", want: "/etc/kubernetes/kops/config/pki/kubelet", ok: true},
		{relative: "pki/compiled/keyset-1", want: "/etc/kubernetes/kops/config/pki/etcd/compiled/keyset-1", ok: true},
		{relative: "addons/bootstrap-channel.yaml", want: "/etc/kubernetes/kops/config/addons/bootstrap-channel.yaml", ok: true},
		{relative: "backups/foo", ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.relative, func(t *testing.T) {
			got, ok := MirrorClusterRelativePath(tc.relative)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
