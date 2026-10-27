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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/kops/pkg/apis/kops"
)

// TestFetchChartFromRepository serves a minimal Helm repository and checks the index lookup, relative
// URL resolution and digest verification.
func TestFetchChartFromRepository(t *testing.T) {
	archive := []byte("not really a tgz, but the bytes are all that matter here")
	sum := sha256.Sum256(archive)
	digest := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/charts/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `apiVersion: v1
entries:
  good:
  - version: 1.0.0
    urls:
    - good-1.0.0.tgz
    digest: %s
  - version: 2.0.0
    urls:
    - good-2.0.0.tgz
    digest: sha256:%s
  absolute:
  - version: 1.0.0
    urls:
    - %s/elsewhere/absolute-1.0.0.tgz
  tampered:
  - version: 1.0.0
    urls:
    - good-1.0.0.tgz
    digest: 0000000000000000000000000000000000000000000000000000000000000000
`, digest, digest, "http://"+r.Host)
	})
	for _, p := range []string{"/charts/good-1.0.0.tgz", "/charts/good-2.0.0.tgz", "/elsewhere/absolute-1.0.0.tgz"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(archive) })
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	repo := server.URL + "/charts/"

	tests := []struct {
		name    string
		chart   kops.AddonChartSpec
		wantErr string
	}{
		{name: "relative url", chart: kops.AddonChartSpec{Repository: repo, Name: "good", Version: "1.0.0"}},
		{name: "prefixed digest", chart: kops.AddonChartSpec{Repository: repo, Name: "good", Version: "2.0.0"}},
		{name: "absolute url, no digest", chart: kops.AddonChartSpec{Repository: repo, Name: "absolute", Version: "1.0.0"}},
		{name: "unknown version", chart: kops.AddonChartSpec{Repository: repo, Name: "good", Version: "3.0.0"}, wantErr: "version 3.0.0 of chart good not found"},
		{name: "unknown chart", chart: kops.AddonChartSpec{Repository: repo, Name: "missing", Version: "1.0.0"}, wantErr: "not found"},
		{name: "digest mismatch", chart: kops.AddonChartSpec{Repository: repo, Name: "tampered", Version: "1.0.0"}, wantErr: "lists digest"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fetchChartFromRepository(context.Background(), &tc.chart)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != string(archive) {
				t.Fatalf("unexpected archive contents %q", got)
			}
		})
	}
}

func TestEnsureChartNamespace(t *testing.T) {
	no := false
	manifest := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n  namespace: x\n")
	tests := []struct {
		name     string
		chart    kops.AddonChartSpec
		manifest string
		prepends bool
	}{
		{name: "custom namespace", chart: kops.AddonChartSpec{Namespace: "x"}, manifest: string(manifest), prepends: true},
		{name: "default namespace is kube-system", chart: kops.AddonChartSpec{}, manifest: string(manifest), prepends: false},
		{name: "createNamespace false", chart: kops.AddonChartSpec{Namespace: "x", CreateNamespace: &no}, manifest: string(manifest), prepends: false},
		{name: "chart ships namespace", chart: kops.AddonChartSpec{Namespace: "x"}, manifest: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: x\n", prepends: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ensureChartNamespace([]byte(tc.manifest), &tc.chart)
			if err != nil {
				t.Fatal(err)
			}
			prepended := strings.HasPrefix(string(got), "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: x\n---\n")
			if prepended != tc.prepends {
				t.Fatalf("prepended=%v, want %v; output:\n%s", prepended, tc.prepends, got)
			}
		})
	}
}
