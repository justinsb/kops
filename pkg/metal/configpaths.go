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
	"path"
	"strings"

	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/nodeup"
)

// LocalConfigRoot is where bare-metal nodes mirror cluster configuration.
const LocalConfigRoot = "/etc/kubernetes/kops/config"

// LocalConfigFileURL returns the file:// URL for LocalConfigRoot.
func LocalConfigFileURL() string {
	return "file://" + LocalConfigRoot
}

// KopsControllerStorePaths returns the on-host configBase and secretStore paths
// for kops-controller on bare metal.
func KopsControllerStorePaths(cluster *kops.Cluster) (configBase string, secretStore string) {
	_ = cluster
	return LocalConfigRoot, path.Join(LocalConfigRoot, "pki")
}

// MirrorClusterRelativePath maps a path relative to the cluster state store to
// the on-host layout used by toolbox enroll.
func MirrorClusterRelativePath(relativePath string) (string, bool) {
	relativePath = strings.TrimPrefix(relativePath, "/")
	switch {
	case relativePath == "config":
		return path.Join(LocalConfigRoot, "config"), true
	case strings.HasPrefix(relativePath, "instancegroup/"):
		return path.Join(LocalConfigRoot, relativePath), true
	case strings.HasPrefix(relativePath, "secrets/"):
		return path.Join(LocalConfigRoot, "pki", strings.TrimPrefix(relativePath, "secrets/")), true
	case strings.HasPrefix(relativePath, "pki/"):
		return path.Join(LocalConfigRoot, "pki", "etcd", strings.TrimPrefix(relativePath, "pki/")), true
	case strings.HasPrefix(relativePath, "igconfig/"):
		return path.Join(LocalConfigRoot, relativePath), true
	case strings.HasPrefix(relativePath, "addons/"):
		return path.Join(LocalConfigRoot, relativePath), true
	default:
		return "", false
	}
}

// RemapNodeupConfigPaths rewrites remote state-store paths in a nodeup.Config to
// on-host locations for bare-metal nodes.
func RemapNodeupConfigPaths(cfg *nodeup.Config) {
	if cfg == nil {
		return
	}
	if cfg.ConfigStore != nil {
		cfg.ConfigStore.Base = LocalConfigRoot
		cfg.ConfigStore.Secrets = path.Join(LocalConfigRoot, "pki")
		cfg.ConfigStore.Keypairs = path.Join(LocalConfigRoot, "pki", "etcd")
	}
	if cfg.ChannelsManifest != "" {
		cfg.ChannelsManifest = remapRemoteStateFile(cfg.ChannelsManifest, LocalConfigRoot)
	}
	for i := range cfg.EtcdManifests {
		cfg.EtcdManifests[i] = remapRemoteStateFile(cfg.EtcdManifests[i], LocalConfigRoot)
	}
}

func remapRemoteStateFile(remotePath string, destDir string) string {
	for _, prefix := range []string{"s3://", "gs://"} {
		if strings.HasPrefix(remotePath, prefix) {
			relativePath := strings.TrimPrefix(remotePath, prefix)
			if i := strings.Index(relativePath, "/"); i >= 0 {
				relativePath = relativePath[i+1:]
			}
			if dest, ok := MirrorClusterRelativePath(relativePath); ok {
				return dest
			}
			return path.Join(destDir, relativePath)
		}
	}
	return remotePath
}
