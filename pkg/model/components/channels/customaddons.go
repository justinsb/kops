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
	"strings"
	"sync"

	"k8s.io/kops/channels/pkg/api"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/kubemanifest"
	"k8s.io/kops/upup/pkg/fi/utils"
	"k8s.io/kops/util/pkg/vfs"
	"sigs.k8s.io/yaml"
)

// CustomAddon is a resolved entry of cluster.Spec.Addons.
//
// Plain manifests are fetched at `kops update cluster` time, verified against spec.sha256 when set,
// and copied ("snapshotted") into the state store under a content-addressed path. The kops-channels
// static pod is then pointed at the copy, so the control plane never needs network access to the
// original URL, the applied content is exactly what was reviewed, and changing the content changes
// the static pod manifest, which rolls the control plane.
//
// kOps `Addons` indexes are not snapshotted: their manifest paths are relative to the index, so the
// index is referenced in place as before.
type CustomAddon struct {
	// Spec is the entry from cluster.Spec.Addons.
	Spec kops.AddonSpec
	// Name is the channel addon name. It is "manifest-" plus the first 12 hex characters of the
	// SHA-256 of the manifest URL, which is what kops-channels itself derives for a plain manifest,
	// so clusters keep the same addon identity when kOps starts snapshotting.
	Name string
	// IsIndex is true when the source is a kOps Addons index.
	IsIndex bool
	// Content is the manifest content (plain manifests only).
	Content []byte
	// ContentHash is the hex SHA-256 of Content (plain manifests only).
	ContentHash string
}

// SnapshotDir is the state-store directory (relative to the config base) holding the snapshot of
// this addon: an `Addons` index named addons.yaml plus manifest.yaml.
func (a *CustomAddon) SnapshotDir() string {
	return "addons/custom/" + a.Name + "/" + a.ContentHash
}

// SnapshotIndex renders the `Addons` index that points kops-channels at the snapshotted manifest.
// The fields match what kops-channels would synthesize for a plain manifest fetched directly.
func (a *CustomAddon) SnapshotIndex() ([]byte, error) {
	name := a.Name
	manifest := "manifest.yaml"
	index := &api.Addons{}
	index.Kind = "Addons"
	index.ObjectMeta.Name = name
	index.Spec.Addons = []*api.AddonSpec{{
		Name:         &name,
		Manifest:     &manifest,
		ManifestHash: a.ContentHash,
	}}
	return yaml.Marshal(index)
}

// customAddonCache memoizes resolved addons for the life of the process: cloudup resolves them
// from more than one builder and we do not want to fetch (or fail) twice.
var customAddonCache sync.Map

// ResolveCustomAddons fetches and verifies every entry of cluster.Spec.Addons.
func ResolveCustomAddons(cluster *kops.Cluster) ([]*CustomAddon, error) {
	var out []*CustomAddon
	for i := range cluster.Spec.Addons {
		spec := cluster.Spec.Addons[i]
		key := cacheKey(cluster, spec)
		if cached, ok := customAddonCache.Load(key); ok {
			out = append(out, cached.(*CustomAddon))
			continue
		}
		chartRenderCluster = cluster
		addon, err := resolveCustomAddon(spec)
		if err != nil {
			return nil, fmt.Errorf("spec.addons[%d]: %w", i, err)
		}
		customAddonCache.Store(key, addon)
		out = append(out, addon)
	}
	return out, nil
}

// chartRenderCluster is the cluster being resolved, made available to the chart renderer (which
// needs e.g. the kubernetes version). Resolution is not concurrent.
var chartRenderCluster *kops.Cluster

// cacheKey identifies an addon entry for memoization: everything that influences the result.
func cacheKey(cluster *kops.Cluster, spec kops.AddonSpec) string {
	if spec.Chart == nil {
		return "manifest|" + spec.Manifest + "|" + spec.SHA256
	}
	c := spec.Chart
	return strings.Join([]string{"chart", cluster.Name, cluster.Spec.KubernetesVersion, c.Repository, c.Name, c.Version, c.SHA256, c.ReleaseName, c.Namespace, fmt.Sprint(c.CreateNamespace != nil && !*c.CreateNamespace), fmt.Sprint(c.IncludeCRDs != nil && !*c.IncludeCRDs), c.Values}, "|")
}

// ResetCustomAddonCache clears the memoized addons; for tests.
func ResetCustomAddonCache() {
	customAddonCache.Range(func(k, _ any) bool {
		customAddonCache.Delete(k)
		return true
	})
}

func resolveCustomAddon(spec kops.AddonSpec) (*CustomAddon, error) {
	if spec.Chart != nil {
		return resolveChartAddon(spec)
	}
	if spec.Manifest == "" {
		return nil, fmt.Errorf("one of manifest and chart is required")
	}
	locationHash, err := utils.HashString(spec.Manifest)
	if err != nil {
		return nil, err
	}
	addon := &CustomAddon{Spec: spec, Name: "manifest-" + locationHash[:12]}

	content, err := vfs.Context.ReadFile(spec.Manifest)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %q: %w", spec.Manifest, err)
	}
	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])
	if spec.SHA256 != "" && !strings.EqualFold(spec.SHA256, contentHash) {
		return nil, fmt.Errorf("manifest %q has sha256 %s, expected %s", spec.Manifest, contentHash, spec.SHA256)
	}

	objects, err := kubemanifest.LoadObjectsFrom(content)
	if err != nil {
		return nil, fmt.Errorf("parsing manifest %q: %w", spec.Manifest, err)
	}
	// Same test kops-channels uses to tell an Addons index from a plain manifest.
	if len(objects) != 0 {
		if gvk := objects[0].GroupVersionKind(); gvk.Kind == "Addons" && gvk.Group == "" && gvk.Version == "" {
			if strings.HasPrefix(spec.Manifest, "file://") {
				return nil, fmt.Errorf("manifest %q is an Addons index served from a local file; the control plane cannot read it (plain manifests are copied to the state store, indexes are not)", spec.Manifest)
			}
			addon.IsIndex = true
			return addon, nil
		}
	}

	addon.Content = content
	addon.ContentHash = contentHash
	return addon, nil
}

// resolveChartAddon renders a Helm chart and wraps the result as a snapshotted custom addon. The addon
// name derives from the repository and chart name only, so bumping the version is an update of the
// same addon rather than a new one.
func resolveChartAddon(spec kops.AddonSpec) (*CustomAddon, error) {
	chart := spec.Chart
	locationHash, err := utils.HashString(strings.TrimSuffix(chart.Repository, "/") + "/" + chart.Name)
	if err != nil {
		return nil, err
	}
	addon := &CustomAddon{Spec: spec, Name: "chart-" + locationHash[:12]}

	rendered, err := chartRenderer.Render(context.TODO(), chartRenderCluster, chart)
	if err != nil {
		return nil, err
	}
	content, err := ensureChartNamespace(rendered, chart)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(content)
	addon.Content = content
	addon.ContentHash = hex.EncodeToString(sum[:])
	return addon, nil
}

// ChannelURLs returns the channel list for the kops-channels static pod: the bootstrap channel
// followed by one entry per custom addon (the snapshot index for plain manifests, the source URL
// for Addons indexes).
func ChannelURLs(cluster *kops.Cluster) ([]string, error) {
	configBase, err := vfs.Context.BuildVfsPath(cluster.Spec.ConfigStore.Base)
	if err != nil {
		return nil, fmt.Errorf("parsing configStore.base %q: %w", cluster.Spec.ConfigStore.Base, err)
	}
	// Built via vfs path joining so toolbox_enroll's identity comparison stays byte-identical.
	channels := []string{configBase.Join("addons", "bootstrap-channel.yaml").Path()}

	addons, err := ResolveCustomAddons(cluster)
	if err != nil {
		return nil, err
	}
	for _, addon := range addons {
		if addon.IsIndex {
			channels = append(channels, addon.Spec.Manifest)
			continue
		}
		channels = append(channels, configBase.Join(addon.SnapshotDir(), "addons.yaml").Path())
	}
	return channels, nil
}

// ChannelListHash returns a hash of the channel list, or "" when there are no custom addons. It is
// recorded in the nodeup config so that a change to the custom addons changes the instance
// configuration and `kops rolling-update cluster` knows control-plane nodes need replacing.
func ChannelListHash(cluster *kops.Cluster) (string, error) {
	if len(cluster.Spec.Addons) == 0 {
		return "", nil
	}
	channels, err := ChannelURLs(cluster)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join(channels, "\n")))
	return hex.EncodeToString(sum[:]), nil
}
