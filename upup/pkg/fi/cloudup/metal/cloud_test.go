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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kops/pkg/apis/kops"
)

func buildNode(name string, labels map[string]string, kubeletVersion string) v1.Node {
	return v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				KubeletVersion: kubeletVersion,
			},
		},
	}
}

func TestGetCloudGroups(t *testing.T) {
	cluster := &kops.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metal.k8s.local",
		},
		Spec: kops.ClusterSpec{
			KubernetesVersion: "1.35.6",
		},
	}

	controlPlaneIG := &kops.InstanceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control-plane-main",
		},
		Spec: kops.InstanceGroupSpec{
			Role: kops.InstanceGroupRoleControlPlane,
		},
	}
	nodesIG := &kops.InstanceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nodes-main",
		},
		Spec: kops.InstanceGroupSpec{
			Role: kops.InstanceGroupRoleNode,
		},
	}

	controlPlaneLabels := map[string]string{"node-role.kubernetes.io/control-plane": ""}

	nodes := []v1.Node{
		buildNode("cp1", controlPlaneLabels, "v1.35.6"),
		buildNode("node1", nil, "v1.35.6"),
		buildNode("node2", nil, "v1.35.0"),
	}

	cloud, err := NewCloud()
	if err != nil {
		t.Fatalf("building cloud: %v", err)
	}

	groups, err := cloud.GetCloudGroups(cluster, []*kops.InstanceGroup{controlPlaneIG, nodesIG}, false, nodes)
	if err != nil {
		t.Fatalf("GetCloudGroups failed: %v", err)
	}

	cpGroup := groups["control-plane-main"]
	if cpGroup == nil {
		t.Fatalf("did not find control-plane-main group")
	}
	if got, want := len(cpGroup.Ready), 1; got != want {
		t.Errorf("control-plane Ready count: got %d, want %d", got, want)
	}
	if got, want := len(cpGroup.NeedUpdate), 0; got != want {
		t.Errorf("control-plane NeedUpdate count: got %d, want %d", got, want)
	}

	nodesGroup := groups["nodes-main"]
	if nodesGroup == nil {
		t.Fatalf("did not find nodes-main group")
	}
	if got, want := len(nodesGroup.Ready), 1; got != want {
		t.Errorf("nodes Ready count: got %d, want %d", got, want)
	}
	if len(nodesGroup.Ready) > 0 && nodesGroup.Ready[0].ID != "node1" {
		t.Errorf("expected node1 to be Ready, got %q", nodesGroup.Ready[0].ID)
	}
	if got, want := len(nodesGroup.NeedUpdate), 1; got != want {
		t.Errorf("nodes NeedUpdate count: got %d, want %d", got, want)
	}
	if len(nodesGroup.NeedUpdate) > 0 && nodesGroup.NeedUpdate[0].ID != "node2" {
		t.Errorf("expected node2 to NeedUpdate, got %q", nodesGroup.NeedUpdate[0].ID)
	}

	if got, want := nodesGroup.TargetSize, 2; got != want {
		t.Errorf("nodes TargetSize: got %d, want %d", got, want)
	}
}

func TestGetCloudGroupsInstanceGroupLabel(t *testing.T) {
	cluster := &kops.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "metal.k8s.local",
		},
		Spec: kops.ClusterSpec{
			KubernetesVersion: "1.35.6",
		},
	}

	igA := &kops.InstanceGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "nodes-a"},
		Spec:       kops.InstanceGroupSpec{Role: kops.InstanceGroupRoleNode},
	}
	igB := &kops.InstanceGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "nodes-b"},
		Spec:       kops.InstanceGroupSpec{Role: kops.InstanceGroupRoleNode},
	}

	nodes := []v1.Node{
		buildNode("node1", map[string]string{"kops.k8s.io/instancegroup": "nodes-a"}, "v1.35.6"),
		buildNode("node2", nil, "v1.35.6"),
	}

	cloud, err := NewCloud()
	if err != nil {
		t.Fatalf("building cloud: %v", err)
	}

	groups, err := cloud.GetCloudGroups(cluster, []*kops.InstanceGroup{igA, igB}, false, nodes)
	if err != nil {
		t.Fatalf("GetCloudGroups failed: %v", err)
	}

	// node1 is labeled for nodes-a, so should only appear there.
	// node2 is unlabeled, so (with role-based matching) appears in both groups.
	if got, want := len(groups["nodes-a"].Ready), 2; got != want {
		t.Errorf("nodes-a Ready count: got %d, want %d", got, want)
	}
	if got, want := len(groups["nodes-b"].Ready), 1; got != want {
		t.Errorf("nodes-b Ready count: got %d, want %d", got, want)
	}
	if len(groups["nodes-b"].Ready) > 0 && groups["nodes-b"].Ready[0].ID != "node2" {
		t.Errorf("expected only node2 in nodes-b, got %q", groups["nodes-b"].Ready[0].ID)
	}
}

func TestNodeMatchesKubernetesVersion(t *testing.T) {
	grid := []struct {
		kubeletVersion    string
		kubernetesVersion string
		want              bool
	}{
		{"v1.35.6", "1.35.6", true},
		{"v1.35.0", "1.35.6", false},
		{"v1.34.1", "1.35.6", false},
		// Unparseable versions are conservatively treated as up-to-date.
		{"", "1.35.6", true},
		{"v1.35.6", "https://example.com/version", true},
	}

	for _, g := range grid {
		node := buildNode("node1", nil, g.kubeletVersion)
		if got := nodeMatchesKubernetesVersion(&node, g.kubernetesVersion); got != g.want {
			t.Errorf("nodeMatchesKubernetesVersion(kubelet=%q, cluster=%q): got %v, want %v", g.kubeletVersion, g.kubernetesVersion, got, g.want)
		}
	}
}
