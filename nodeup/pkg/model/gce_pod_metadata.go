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
	"k8s.io/klog/v2"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/systemd"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/nodeup/nodetasks"
)

// GCEPodMetadataBuilder blocks Pod access to the Compute Engine metadata server when
// spec.cloudProvider.gce.podCredentials is None.
//
// The block is installed by a systemd unit at boot rather than by anything running in the cluster, so
// that no Pod can ever reach the metadata server, not even one scheduled before cluster components are
// up. Pod connections are redirected to a loopback address with no listener and therefore refused.
// Traffic from the host itself (node daemons and host-network Pods) uses the OUTPUT chain, not
// PREROUTING, and keeps working. Redirecting in the nat table rather than rejecting in FORWARD keeps the
// block effective when a CNI later inserts blanket ACCEPT rules at the top of FORWARD, as Cilium does.
type GCEPodMetadataBuilder struct {
	*NodeupModelContext
}

var _ fi.NodeupModelBuilder = &GCEPodMetadataBuilder{}

const (
	// gceMetadataServerIP is the fixed address of the Compute Engine metadata server.
	gceMetadataServerIP = "169.254.169.254"
	// gceMetadataSinkIP is the loopback alias Pod metadata traffic is redirected to. It is the address
	// GKE's metadata server listens on, so a future filtering proxy can simply listen there.
	gceMetadataSinkIP = "169.254.169.252"
)

// gceBlockPodMetadataScript installs the loopback alias and the NAT rules. It is idempotent so the unit
// can be re-run safely. Port 80 is the current metadata API and 8080 the legacy one.
const gceBlockPodMetadataScript = `#!/bin/bash
# Built by kOps - do not edit
set -e

ip addr replace ` + gceMetadataSinkIP + `/32 dev lo scope host

for ports in 80:988 8080:987; do
  dport="${ports%%:*}"
  target="${ports##*:}"
  rule=(-p tcp -d ` + gceMetadataServerIP + ` --dport "${dport}" -m comment --comment "kops podCredentials=None: pod traffic to the metadata server is refused" -j DNAT --to-destination ` + gceMetadataSinkIP + `:"${target}")
  if ! iptables -w -t nat -C PREROUTING "${rule[@]}" 2>/dev/null; then
    iptables -w -t nat -I PREROUTING "${rule[@]}"
  fi
done
`

// Build installs the block when Pods are not to receive the node's credentials on a GCE cluster.
func (b *GCEPodMetadataBuilder) Build(c *fi.NodeupModelBuilderContext) error {
	if b.CloudProvider() != kops.CloudProviderGCE || !b.NodeupConfig.GCEBlockPodMetadataAccess {
		return nil
	}

	c.AddTask(&nodetasks.File{
		Path:     "/opt/kops/bin/gce-block-pod-metadata",
		Contents: fi.NewStringResource(gceBlockPodMetadataScript),
		Type:     nodetasks.FileType_File,
		Mode:     s("0755"),
	})

	manifest := &systemd.Manifest{}
	manifest.Set("Unit", "Description", "Block pod access to the GCE metadata server")
	manifest.Set("Unit", "Documentation", "https://github.com/kubernetes/kops")
	// Before the network is up, and therefore before kubelet and containerd start any Pod.
	manifest.Set("Unit", "Before", "network.target")
	manifest.Set("Service", "Type", "oneshot")
	manifest.Set("Service", "RemainAfterExit", "yes")
	manifest.Set("Service", "ExecStart", "/opt/kops/bin/gce-block-pod-metadata")
	manifest.Set("Install", "WantedBy", "basic.target")

	manifestString := manifest.Render()
	klog.V(8).Infof("Built service manifest %q\n%s", "gce-block-pod-metadata.service", manifestString)

	service := &nodetasks.Service{
		Name:       "gce-block-pod-metadata.service",
		Definition: s(manifestString),
	}
	service.InitDefaults()
	c.AddTask(service)

	return nil
}
