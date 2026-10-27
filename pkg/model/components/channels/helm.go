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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/pkg/sandbox"
	"k8s.io/kops/util/pkg/vfs"
	"sigs.k8s.io/yaml"
)

const (
	// helmVersion is the Helm release used to render charts. It is pinned so that every operator
	// renders the same output for the same chart and values.
	helmVersion = "v3.22.1"
	// helmImage is the container image used by image-based sandboxes, pinned by digest.
	helmImage = "docker.io/alpine/helm:3.22.1@sha256:8191701ad1f82ee3695c87211fcbfe6d7b1bb7df92ac1c8de9ccdb0995a6a2c1"
	// helmDownloadBase hosts the official Helm release tarballs and their checksum files.
	helmDownloadBase = "https://get.helm.sh/"
)

// ChartRenderer renders a Helm chart into a manifest.
type ChartRenderer interface {
	Render(ctx context.Context, cluster *kops.Cluster, chart *kops.AddonChartSpec) ([]byte, error)
}

var chartRenderer ChartRenderer = &helmRenderer{}

// SetChartRendererForTests replaces the renderer and returns a function restoring the previous one.
func SetChartRendererForTests(r ChartRenderer) (restore func()) {
	previous := chartRenderer
	chartRenderer = r
	return func() { chartRenderer = previous }
}

// helmRenderer fetches a chart and runs `helm template` in a sandbox.
type helmRenderer struct{}

func (h *helmRenderer) Render(ctx context.Context, cluster *kops.Cluster, chart *kops.AddonChartSpec) ([]byte, error) {
	executor, err := sandbox.Detect()
	if err != nil {
		return nil, err
	}
	tool := sandbox.Tool{Name: "helm", Image: helmImage}
	switch executor.Name() {
	case sandbox.NameSeatbelt, sandbox.NameNone:
		tool.HostPath, err = ensureHelmBinary(ctx)
		if err != nil {
			return nil, err
		}
	}
	klog.V(2).Infof("rendering chart %s/%s %s with helm %s in sandbox %q", chart.Repository, chart.Name, chart.Version, helmVersion, executor.Name())

	// Work under ~/.kops rather than the system temp directory: container runtimes on macOS only
	// share a few host paths (the home directory among them) with their VM, and macOS temp paths are
	// not one of them.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	workBase := filepath.Join(home, ".kops", "tmp")
	if err := os.MkdirAll(workBase, 0o700); err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp(workBase, "helm-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	// Seatbelt profiles match on resolved paths, and macOS temp directories live behind symlinks.
	if work, err = filepath.EvalSymlinks(work); err != nil {
		return nil, err
	}
	chartDir := filepath.Join(work, "chart")
	valuesDir := filepath.Join(work, "values")
	homeDir := filepath.Join(work, "home")
	for _, d := range []string{chartDir, valuesDir, homeDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	valuesPath := filepath.Join(valuesDir, "values.yaml")
	if err := os.WriteFile(valuesPath, []byte(chart.Values), 0o600); err != nil {
		return nil, err
	}
	// Keep every bit of Helm state inside the working directory: no reading of the operator's
	// repositories, plugins or registry logins.
	env := map[string]string{
		"HOME":                   homeDir,
		"HELM_CONFIG_HOME":       filepath.Join(homeDir, "config"),
		"HELM_CACHE_HOME":        filepath.Join(homeDir, "cache"),
		"HELM_DATA_HOME":         filepath.Join(homeDir, "data"),
		"HELM_REPOSITORY_CONFIG": filepath.Join(homeDir, "config", "repositories.yaml"),
		"HELM_REPOSITORY_CACHE":  filepath.Join(homeDir, "cache", "repository"),
		"HELM_REGISTRY_CONFIG":   filepath.Join(homeDir, "config", "registry.json"),
		"HELM_PLUGINS":           filepath.Join(homeDir, "data", "plugins"),
	}

	// 1. Fetch the chart archive.
	var archive string
	if strings.HasPrefix(chart.Repository, "oci://") {
		// Only Helm speaks the OCI registry protocol, so this step runs Helm with the network on. It
		// sees nothing of the host except the working directory, and needs to write there.
		var pullErr bytes.Buffer
		if err := executor.Run(ctx, &sandbox.Command{
			Tool:    tool,
			Args:    []string{"pull", strings.TrimSuffix(chart.Repository, "/") + "/" + chart.Name, "--version", chart.Version, "--destination", chartDir},
			Env:     env,
			Dir:     work,
			Mounts:  []sandbox.Mount{{Path: work}},
			Network: true,
			Stdout:  io.Discard,
			Stderr:  &pullErr,
		}); err != nil {
			return nil, fmt.Errorf("fetching chart %s from %s: %w\n%s", chart.Name, chart.Repository, err, strings.TrimSpace(pullErr.String()))
		}
		archives, err := filepath.Glob(filepath.Join(chartDir, "*.tgz"))
		if err != nil || len(archives) != 1 {
			return nil, fmt.Errorf("expected exactly one chart archive after fetching %s, found %d", chart.Name, len(archives))
		}
		archive = archives[0]
	} else {
		// Plain repositories are an index.yaml plus archives over HTTPS, which kOps can fetch itself.
		// Helm then never needs the network, and the sandbox only mounts read-only inputs, which also
		// suits container runtimes that share the home directory read-only.
		data, err := fetchChartFromRepository(ctx, chart)
		if err != nil {
			return nil, fmt.Errorf("fetching chart %s from %s: %w", chart.Name, chart.Repository, err)
		}
		archive = filepath.Join(chartDir, chart.Name+"-"+chart.Version+".tgz")
		if err := os.WriteFile(archive, data, 0o600); err != nil {
			return nil, err
		}
	}
	if chart.SHA256 != "" {
		data, err := os.ReadFile(archive)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		if actual := hex.EncodeToString(sum[:]); !strings.EqualFold(actual, chart.SHA256) {
			return nil, fmt.Errorf("chart %s %s has sha256 %s, expected %s", chart.Name, chart.Version, actual, chart.SHA256)
		}
	}

	// 2. Render. Chart templates are third-party code: no network, every mount read-only
	//    (`helm template` with a local archive writes nothing, not even under HOME).
	release := chart.ReleaseName
	if release == "" {
		release = chart.Name
	}
	args := []string{"template", release, archive,
		"--namespace", chartNamespace(chart),
		"--values", valuesPath,
		"--kube-version", strings.TrimPrefix(cluster.Spec.KubernetesVersion, "v"),
		"--skip-tests",
	}
	if chart.IncludeCRDs == nil || *chart.IncludeCRDs {
		args = append(args, "--include-crds")
	}
	var stdout, stderr bytes.Buffer
	if err := executor.Run(ctx, &sandbox.Command{
		Tool:   tool,
		Args:   args,
		Env:    env,
		Dir:    work,
		Mounts: []sandbox.Mount{{Path: chartDir, ReadOnly: true}, {Path: valuesDir, ReadOnly: true}, {Path: homeDir, ReadOnly: true}},
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return nil, fmt.Errorf("rendering chart %s: %w\n%s", chart.Name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// repositoryIndex is the subset of a Helm repository index.yaml that locating a chart needs.
type repositoryIndex struct {
	Entries map[string][]repositoryIndexEntry `json:"entries"`
}

type repositoryIndexEntry struct {
	Version string   `json:"version"`
	URLs    []string `json:"urls"`
	Digest  string   `json:"digest"`
}

// fetchChartFromRepository downloads a chart archive from a plain (https) Helm repository by reading
// its index.yaml, the same lookup `helm pull --repo` performs.
func fetchChartFromRepository(ctx context.Context, chart *kops.AddonChartSpec) ([]byte, error) {
	base := strings.TrimSuffix(chart.Repository, "/")
	indexURL := base + "/index.yaml"
	indexData, err := vfs.Context.ReadFile(indexURL)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", indexURL, err)
	}
	var index repositoryIndex
	if err := yaml.Unmarshal(indexData, &index); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", indexURL, err)
	}
	var entry *repositoryIndexEntry
	for i := range index.Entries[chart.Name] {
		if index.Entries[chart.Name][i].Version == chart.Version {
			entry = &index.Entries[chart.Name][i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("version %s of chart %s not found in %s", chart.Version, chart.Name, indexURL)
	}
	if len(entry.URLs) == 0 {
		return nil, fmt.Errorf("%s lists no download URL for chart %s %s", indexURL, chart.Name, chart.Version)
	}
	archiveURL := entry.URLs[0]
	if !strings.Contains(archiveURL, "://") {
		// Relative to the repository, per the Helm chart repository guide.
		archiveURL = base + "/" + strings.TrimPrefix(archiveURL, "/")
	}
	data, err := vfs.Context.ReadFile(archiveURL)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", archiveURL, err)
	}
	// The index carries the archive digest; a mismatch means the repository is inconsistent (or
	// tampered with), which is worth refusing even when the spec does not pin a sha256.
	if entry.Digest != "" {
		sum := sha256.Sum256(data)
		if actual := hex.EncodeToString(sum[:]); !strings.EqualFold(actual, strings.TrimPrefix(entry.Digest, "sha256:")) {
			return nil, fmt.Errorf("%s has sha256 %s but %s lists digest %s", archiveURL, actual, indexURL, entry.Digest)
		}
	}
	return data, nil
}

// chartNamespace returns the namespace a chart is rendered for.
func chartNamespace(chart *kops.AddonChartSpec) string {
	if chart.Namespace != "" {
		return chart.Namespace
	}
	return "kube-system"
}

// ensureChartNamespace prepends a Namespace object when the chart expects its namespace to exist
// already, which is what `helm install --create-namespace` would do.
func ensureChartNamespace(manifest []byte, chart *kops.AddonChartSpec) ([]byte, error) {
	if chart.CreateNamespace != nil && !*chart.CreateNamespace {
		return manifest, nil
	}
	ns := chartNamespace(chart)
	switch ns {
	case "default", "kube-system", "kube-public", "kube-node-lease":
		return manifest, nil
	}
	objects, err := kubemanifest.LoadObjectsFrom(manifest)
	if err != nil {
		return nil, fmt.Errorf("parsing rendered chart: %w", err)
	}
	for _, o := range objects {
		u := o.ToUnstructured()
		if u.GetKind() == "Namespace" && u.GetName() == ns {
			return manifest, nil
		}
	}
	header := fmt.Sprintf("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n---\n", ns)
	return append([]byte(header), manifest...), nil
}

// ensureHelmBinary downloads the pinned Helm release for this platform into ~/.kops/tools, verifying
// it against the published checksum, and returns its path.
func ensureHelmBinary(ctx context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kops", "tools", "helm", helmVersion)
	bin := filepath.Join(dir, "helm")
	if st, err := os.Stat(bin); err == nil && st.Mode().IsRegular() {
		return bin, nil
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	name := fmt.Sprintf("helm-%s-%s.tar.gz", helmVersion, platform)
	klog.Infof("downloading helm %s for %s", helmVersion, platform)
	archive, err := vfs.Context.ReadFile(helmDownloadBase + name)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", name, err)
	}
	sumFile, err := vfs.Context.ReadFile(helmDownloadBase + name + ".sha256sum")
	if err != nil {
		return "", fmt.Errorf("downloading checksum for %s: %w", name, err)
	}
	expected := strings.Fields(string(sumFile))
	sum := sha256.Sum256(archive)
	if len(expected) == 0 || !strings.EqualFold(expected[0], hex.EncodeToString(sum[:])) {
		return "", fmt.Errorf("checksum mismatch for %s", name)
	}
	helmBytes, err := extractFromTarGz(archive, platform+"/helm")
	if err != nil {
		return "", fmt.Errorf("extracting %s: %w", name, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp := bin + ".tmp"
	if err := os.WriteFile(tmp, helmBytes, 0o600); err != nil {
		return "", err
	}
	// Executable, but private to the operator: nothing else needs to run this copy.
	if err := os.Chmod(tmp, 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, bin); err != nil {
		return "", err
	}
	return bin, nil
}

func extractFromTarGz(archive []byte, member string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in archive", member)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == member || hdr.Name == "./"+member {
			return io.ReadAll(tr)
		}
	}
}
