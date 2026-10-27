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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/util/pkg/vfs"
)

func testCluster() *kops.Cluster {
	c := &kops.Cluster{}
	c.Name = "gw.k8s.local"
	c.Spec.KubernetesVersion = "1.37.1"
	c.Spec.CloudProvider.GCE = &kops.GCESpec{Project: "my-project"}
	c.Spec.Networking.Subnets = []kops.ClusterSubnetSpec{{Name: "us-central1", Region: "us-central1"}}
	return c
}

func mustParse(t *testing.T, yaml string) *unstructured.Unstructured {
	t.Helper()
	objects, err := kubemanifest.LoadObjectsFrom([]byte(yaml))
	if err != nil {
		t.Fatalf("parsing test object: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	return objects[0].ToUnstructured()
}

func TestValuesForCluster(t *testing.T) {
	v := ValuesForCluster(testCluster())
	if v.ClusterName != "gw.k8s.local" || v.CloudProvider != "gce" || v.Region != "us-central1" || v.KubernetesVersion != "1.37.1" {
		t.Errorf("unexpected values: %+v", v)
	}
	if v.GCP == nil {
		t.Fatalf("expected GCP values")
	}
	if v.GCP.Project != "my-project" {
		t.Errorf("Project = %q", v.GCP.Project)
	}
	if want := "node-gw-k8s-local@my-project.iam.gserviceaccount.com"; v.GCP.NodeServiceAccount != want {
		t.Errorf("NodeServiceAccount = %q, want %q", v.GCP.NodeServiceAccount, want)
	}
	if want := "control-plane-gw-k8s-local@my-project.iam.gserviceaccount.com"; v.GCP.ControlPlaneServiceAccount != want {
		t.Errorf("ControlPlaneServiceAccount = %q, want %q", v.GCP.ControlPlaneServiceAccount, want)
	}
}

func TestRender(t *testing.T) {
	v := ValuesForCluster(testCluster())
	out, err := Render("t", "name: {{ .ClusterName }} project: {{ .GCP.Project }}", v)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if out != "name: gw.k8s.local project: my-project" {
		t.Errorf("Render = %q", out)
	}
	if _, err := Render("t", "{{ .DoesNotExist }}", v); err == nil {
		t.Errorf("expected an error for an unknown field")
	}
	// Plain text without actions passes through untouched, including braces that are not actions.
	out, err = Render("t", "a: {b}", v)
	if err != nil || out != "a: {b}" {
		t.Errorf("Render plain = %q, %v", out, err)
	}
}

func TestRenderStrings(t *testing.T) {
	v := ValuesForCluster(testCluster())
	in := map[string]interface{}{
		"bindings": []interface{}{
			map[string]interface{}{
				"role":    "roles/dns.admin",
				"members": []interface{}{"serviceAccount:{{ .GCP.NodeServiceAccount }}"},
			},
		},
		"count": int64(3),
	}
	outI, err := RenderStrings("spec", in, v)
	if err != nil {
		t.Fatalf("RenderStrings: %v", err)
	}
	out := outI.(map[string]interface{})
	member := out["bindings"].([]interface{})[0].(map[string]interface{})["members"].([]interface{})[0].(string)
	if member != "serviceAccount:node-gw-k8s-local@my-project.iam.gserviceaccount.com" {
		t.Errorf("member = %q", member)
	}
	if out["count"] != int64(3) {
		t.Errorf("non-string value changed: %v", out["count"])
	}
}

const inlinePackage = `
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata:
  name: hello
spec:
  version: v1
  render: true
  manifest:
    inline: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: hello
        namespace: kube-system
      data:
        cluster: "{{ .ClusterName }}"
`

func TestLoadPackageInline(t *testing.T) {
	ctx := context.Background()
	vfsContext := vfs.NewTestingVFSContext()
	pkg, err := LoadPackage(ctx, vfsContext, ValuesForCluster(testCluster()), mustParse(t, inlinePackage))
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if pkg.Name != "hello" || pkg.Version != "v1" || !pkg.Prune {
		t.Errorf("unexpected package: %+v", pkg)
	}
	if !strings.Contains(string(pkg.Manifest), `cluster: "gw.k8s.local"`) {
		t.Errorf("manifest was not rendered: %s", pkg.Manifest)
	}
}

func TestLoadPackageFromURLWithSHA256(t *testing.T) {
	ctx := context.Background()
	vfsContext := vfs.NewTestingVFSContext()
	dir := t.TempDir()
	manifest := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(manifest))
	good := hex.EncodeToString(sum[:])

	mk := func(sha string, prune string) *unstructured.Unstructured {
		return mustParse(t, `
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata:
  name: cert-manager
spec:
  version: v1.21.3
  prune: `+prune+`
  manifest:
    url: file://`+path+`
    sha256: "`+sha+`"
`)
	}

	pkg, err := LoadPackage(ctx, vfsContext, Values{}, mk(good, "false"))
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	if string(pkg.Manifest) != manifest {
		t.Errorf("manifest mismatch: %q", pkg.Manifest)
	}
	if pkg.Prune {
		t.Errorf("expected prune=false")
	}

	bad := strings.Repeat("0", 64)
	if _, err := LoadPackage(ctx, vfsContext, Values{}, mk(bad, "true")); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("expected a sha256 mismatch error, got %v", err)
	}
}

func TestLoadPackageRejectsBadManifest(t *testing.T) {
	ctx := context.Background()
	u := mustParse(t, `
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata:
  name: broken
spec:
  version: v1
  manifest:
    inline: "this: is: not: yaml: ["
`)
	if _, err := LoadPackage(ctx, vfs.NewTestingVFSContext(), Values{}, u); err == nil {
		t.Errorf("expected an error for an unparseable manifest")
	}
}

func TestParseGCPResource(t *testing.T) {
	v := ValuesForCluster(testCluster())
	u := mustParse(t, `
apiVersion: addons.kops.k8s.io/v1alpha1
kind: GCPResource
metadata:
  name: //serviceusage.googleapis.com/projects/{{ .GCP.Project }}/services/publicca.googleapis.com
spec:
  state: ENABLED
`)
	r, err := ParseGCPResource(v, u)
	if err != nil {
		t.Fatalf("ParseGCPResource: %v", err)
	}
	if r.Service != "serviceusage.googleapis.com" || r.Path != "projects/my-project/services/publicca.googleapis.com" {
		t.Errorf("unexpected parse: %+v", r)
	}
	if r.Spec["state"] != "ENABLED" {
		t.Errorf("spec = %v", r.Spec)
	}
	segs, err := r.PathSegments()
	if err != nil || segs["projects"] != "my-project" || segs["services"] != "publicca.googleapis.com" {
		t.Errorf("PathSegments = %v, %v", segs, err)
	}
}

func TestParseResourceURL(t *testing.T) {
	for _, bad := range []string{"", "projects/p", "//", "//iam.googleapis.com", "//iam.googleapis.com/", "//example.com/projects/p"} {
		if _, _, err := ParseResourceURL(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	s, p, err := ParseResourceURL("//iam.googleapis.com/projects/p/serviceAccounts/a@p.iam.gserviceaccount.com")
	if err != nil || s != "iam.googleapis.com" || p != "projects/p/serviceAccounts/a@p.iam.gserviceaccount.com" {
		t.Errorf("got %q %q %v", s, p, err)
	}
}

func TestPartitionObjects(t *testing.T) {
	objects, err := kubemanifest.LoadObjectsFrom([]byte(inlinePackage + `
---
apiVersion: addons.kops.k8s.io/v1alpha1
kind: GCPResource
metadata:
  name: //serviceusage.googleapis.com/projects/p/services/dns.googleapis.com
spec: {}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: plain
  namespace: default
`))
	if err != nil {
		t.Fatal(err)
	}
	p := PartitionObjects(objects)
	if len(p.ClusterPackages) != 1 || len(p.GCPResources) != 1 || len(p.Other) != 1 {
		t.Errorf("partition = %d/%d/%d", len(p.ClusterPackages), len(p.GCPResources), len(p.Other))
	}
}

func TestValidateObject(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"valid package", inlinePackage, ""},
		{"other group ignored", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n", ""},
		{"missing version", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: ClusterPackage\nmetadata:\n  name: x\nspec:\n  manifest:\n    inline: 'a: b'\n", "version"},
		{"url and inline", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: ClusterPackage\nmetadata:\n  name: x\nspec:\n  version: v1\n  manifest:\n    url: https://example.com/a.yaml\n    inline: 'a: b'\n", "only one"},
		{"http rejected", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: ClusterPackage\nmetadata:\n  name: x\nspec:\n  version: v1\n  manifest:\n    url: http://example.com/a.yaml\n", "http"},
		{"bad sha", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: ClusterPackage\nmetadata:\n  name: x\nspec:\n  version: v1\n  manifest:\n    url: https://example.com/a.yaml\n    sha256: abc\n", "64 hex"},
		{"namespaced package", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: ClusterPackage\nmetadata:\n  name: x\n  namespace: kube-system\nspec:\n  version: v1\n  manifest:\n    inline: 'a: b'\n", "cluster-scoped"},
		{"unknown kind", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: Widget\nmetadata:\n  name: x\n", "Widget"},
		{"wrong version", "apiVersion: addons.kops.k8s.io/v1beta9\nkind: ClusterPackage\nmetadata:\n  name: x\n", "v1beta9"},
		{"gcp bad url", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: GCPResource\nmetadata:\n  name: projects/p\nspec: {}\n", "must start with //"},
		{"gcp templated name ok", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: GCPResource\nmetadata:\n  name: //x.googleapis.com/projects/{{ .GCP.Project }}\nspec: {}\n", ""},
		{"gcp missing spec", "apiVersion: addons.kops.k8s.io/v1alpha1\nkind: GCPResource\nmetadata:\n  name: //x.googleapis.com/projects/p\n", "spec"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateObject(field.NewPath("addons"), mustParse(t, tc.yaml))
			if tc.wantErr == "" {
				if len(errs) != 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
				return
			}
			if len(errs) == 0 || !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, errs)
			}
		})
	}
}
