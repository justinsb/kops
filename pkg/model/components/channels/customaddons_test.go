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

package channels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/util/pkg/vfs"
)

func writeMemfs(t *testing.T, p string, content string) {
	t.Helper()
	path, err := vfs.Context.BuildVfsPath(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := path.WriteFile(context.Background(), strings.NewReader(content), nil); err != nil {
		t.Fatal(err)
	}
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestResolveCustomAddons(t *testing.T) {
	vfs.Context.ResetMemfsContext(true)
	ResetCustomAddonCache()

	plain := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"
	index := "kind: Addons\nmetadata:\n  name: example\nspec:\n  addons:\n  - name: foo\n    manifest: foo/v1.yaml\n"
	writeMemfs(t, "memfs://tests/addons/plain.yaml", plain)
	writeMemfs(t, "memfs://tests/addons/index.yaml", index)

	cluster := &kops.Cluster{}
	cluster.Name = "test.example.com"
	cluster.Spec.ConfigStore.Base = "memfs://tests/state/test.example.com"
	cluster.Spec.Addons = []kops.AddonSpec{
		{Manifest: "memfs://tests/addons/plain.yaml", SHA256: sha(plain)},
		{Manifest: "memfs://tests/addons/index.yaml"},
	}

	addons, err := ResolveCustomAddons(cluster)
	if err != nil {
		t.Fatalf("ResolveCustomAddons: %v", err)
	}
	if len(addons) != 2 {
		t.Fatalf("expected 2 addons, got %d", len(addons))
	}
	p, idx := addons[0], addons[1]
	if p.IsIndex || string(p.Content) != plain || p.ContentHash != sha(plain) {
		t.Errorf("plain manifest not resolved as expected: %+v", p)
	}
	if !strings.HasPrefix(p.Name, "manifest-") || len(p.Name) != len("manifest-")+12 {
		t.Errorf("unexpected addon name %q", p.Name)
	}
	if !idx.IsIndex || idx.Content != nil {
		t.Errorf("index not detected: %+v", idx)
	}
	if got, want := p.SnapshotDir(), "addons/custom/"+p.Name+"/"+sha(plain); got != want {
		t.Errorf("SnapshotDir = %q, want %q", got, want)
	}
	indexYAML, err := p.SnapshotIndex()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kind: Addons", "name: " + p.Name, "manifest: manifest.yaml", "manifestHash: " + sha(plain)} {
		if !strings.Contains(string(indexYAML), want) {
			t.Errorf("snapshot index missing %q:\n%s", want, indexYAML)
		}
	}

	urls, err := ChannelURLs(cluster)
	if err != nil {
		t.Fatal(err)
	}
	wantURLs := []string{
		"memfs://tests/state/test.example.com/addons/bootstrap-channel.yaml",
		"memfs://tests/state/test.example.com/" + p.SnapshotDir() + "/addons.yaml",
		"memfs://tests/addons/index.yaml",
	}
	if strings.Join(urls, ",") != strings.Join(wantURLs, ",") {
		t.Errorf("ChannelURLs = %v, want %v", urls, wantURLs)
	}

	hash, err := ChannelListHash(cluster)
	if err != nil || len(hash) != 64 {
		t.Errorf("ChannelListHash = %q, %v", hash, err)
	}
	cluster.Spec.Addons = nil
	if hash, _ := ChannelListHash(cluster); hash != "" {
		t.Errorf("expected an empty hash without custom addons, got %q", hash)
	}
}

func TestResolveCustomAddonsErrors(t *testing.T) {
	vfs.Context.ResetMemfsContext(true)
	ResetCustomAddonCache()
	plain := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: x\n"
	writeMemfs(t, "memfs://tests/addons/plain.yaml", plain)

	cluster := &kops.Cluster{}
	cluster.Spec.ConfigStore.Base = "memfs://tests/state/c"

	cluster.Spec.Addons = []kops.AddonSpec{{Manifest: "memfs://tests/addons/plain.yaml", SHA256: strings.Repeat("0", 64)}}
	if _, err := ResolveCustomAddons(cluster); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("expected a sha256 mismatch error, got %v", err)
	}
	cluster.Spec.Addons = []kops.AddonSpec{{Manifest: "memfs://tests/addons/missing.yaml"}}
	if _, err := ResolveCustomAddons(cluster); err == nil || !strings.Contains(err.Error(), "spec.addons[0]") {
		t.Errorf("expected a read error naming the entry, got %v", err)
	}
}
