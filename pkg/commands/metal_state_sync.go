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

package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	kopscontrollerconfig "k8s.io/kops/cmd/kops-controller/pkg/config"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/metal"
	"k8s.io/kops/pkg/nodelabels"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kops/util/pkg/vfs"
)

// BuildClusterStateMirror reads the remote cluster state store and returns the
// files that should exist on bare-metal control-plane nodes.
func BuildClusterStateMirror(ctx context.Context, vfsContext *vfs.VFSContext, remoteBase string) (map[string][]byte, error) {
	remotePath, err := vfsContext.BuildVfsPath(remoteBase)
	if err != nil {
		return nil, fmt.Errorf("building vfs path %q: %w", remoteBase, err)
	}

	remoteFiles, err := remotePath.ReadTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading cluster state store %q: %w", remoteBase, err)
	}

	remoteRoot := remotePath.Path()
	files := make(map[string][]byte, len(remoteFiles))
	for _, remoteFile := range remoteFiles {
		filePath := remoteFile.Path()
		if !strings.HasPrefix(filePath, remoteRoot) {
			return nil, fmt.Errorf("unexpected path %q (expected prefix %q)", filePath, remoteRoot)
		}
		relativePath := strings.TrimPrefix(filePath, remoteRoot)
		// igconfig is always rebuilt from the current cluster spec; never mirror from S3.
		if strings.HasPrefix(strings.TrimPrefix(relativePath, "/"), "igconfig/") {
			continue
		}
		destPath, ok := metal.MirrorClusterRelativePath(relativePath)
		if !ok {
			continue
		}

		b, err := remoteFile.ReadFile(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading file %q: %w", filePath, err)
		}
		files[destPath] = b
	}

	return files, nil
}

// appendMetalIGConfigFiles adds nodeupconfig.yaml for each instance group.
// Always rebuild from the current cluster spec so kops-controller serves the
// latest kubernetes version; a stale copy in the remote state store must not win.
func appendMetalIGConfigFiles(ctx context.Context, cluster *kops.Cluster, clientset simple.Clientset, files map[string][]byte) error {
	cloud, err := cloudup.BuildCloud(cluster)
	if err != nil {
		return fmt.Errorf("building cloud: %w", err)
	}

	instanceGroups, err := clientset.InstanceGroupsFor(cluster).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing instance groups: %w", err)
	}

	for i := range instanceGroups.Items {
		ig := &instanceGroups.Items[i]
		destPath := path.Join(metal.LocalConfigRoot, "igconfig", ig.Spec.Role.ToLowerString(), ig.Name, "nodeupconfig.yaml")

		configBuilder := &ConfigBuilder{
			Clientset:         clientset,
			ClusterName:       cluster.Name,
			InstanceGroupName: ig.Name,
			Cloud:             cloud,
			Cluster:           cluster,
		}
		bootstrapData, err := configBuilder.GetBootstrapData(ctx)
		if err != nil {
			return fmt.Errorf("building bootstrap data for instance group %q: %w", ig.Name, err)
		}

		nodeupConfigBytes, err := yaml.Marshal(bootstrapData.NodeupConfig)
		if err != nil {
			return fmt.Errorf("marshalling nodeup config for instance group %q: %w", ig.Name, err)
		}
		files[destPath] = nodeupConfigBytes
		klog.Infof("Generated igconfig for instance group %q at %q", ig.Name, destPath)
	}

	return nil
}

// WriteFilesToHost writes files to a remote host over SSH.
func (s *SSHHost) WriteFilesToHost(ctx context.Context, files map[string][]byte) error {
	for destPath, contents := range files {
		if err := s.writeFile(ctx, destPath, bytes.NewReader(contents)); err != nil {
			return fmt.Errorf("writing file %q over SSH: %w", destPath, err)
		}
	}
	return nil
}

// SyncMetalClusterStateToControlPlane mirrors the cluster state store to every
// control-plane node so kops-controller and kops-channels can serve configuration
// without cloud credentials.
func SyncMetalClusterStateToControlPlane(ctx context.Context, cluster *kops.Cluster, clientset simple.Clientset, k8sClient kubernetes.Interface, sshUser string, sshPort int, out io.Writer) error {
	if cluster.Spec.ConfigStore.Base == "" {
		return fmt.Errorf("cluster %q has no configStore.base", cluster.Name)
	}

	files, err := BuildClusterStateMirror(ctx, clientset.VFSContext(), cluster.Spec.ConfigStore.Base)
	if err != nil {
		return fmt.Errorf("building cluster state mirror: %w", err)
	}
	if err := appendMetalIGConfigFiles(ctx, cluster, clientset, files); err != nil {
		return fmt.Errorf("generating igconfig files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("cluster state store %q was empty", cluster.Spec.ConfigStore.Base)
	}

	if err := writeFilesToControlPlaneNodes(ctx, k8sClient, sshUser, sshPort, files, out); err != nil {
		return err
	}

	if err := patchKopsControllerConfigMap(ctx, k8sClient, cluster); err != nil {
		return fmt.Errorf("patching kops-controller configuration: %w", err)
	}
	if out != nil {
		fmt.Fprintf(out, "Updated kops-controller ConfigMap to use local state store paths\n")
	}

	if err := ensureKopsControllerStateStoreMount(ctx, k8sClient); err != nil {
		return fmt.Errorf("mounting local state store in kops-controller: %w", err)
	}
	if out != nil {
		fmt.Fprintf(out, "Mounted %s into kops-controller\n", metal.LocalConfigRoot)
	}

	// Bounce kops-controller so it reloads configuration from the updated mirror.
	if err := restartKopsControllerPods(ctx, k8sClient); err != nil {
		return fmt.Errorf("restarting kops-controller after state sync: %w", err)
	}

	if out != nil {
		fmt.Fprintf(out, "Restarted kops-controller to pick up updated configuration\n")
	}
	return nil
}

func writeFilesToControlPlaneNodes(ctx context.Context, k8sClient kubernetes.Interface, sshUser string, sshPort int, files map[string][]byte, out io.Writer) error {
	if sshUser == "" {
		sshUser = "root"
	}
	if sshPort == 0 {
		sshPort = 22
	}
	sudo := sshUser != "root"

	nodeList, err := k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: nodelabels.RoleLabelControlPlane20,
	})
	if err != nil {
		return fmt.Errorf("listing control-plane nodes: %w", err)
	}
	if len(nodeList.Items) == 0 {
		return fmt.Errorf("no control-plane nodes found")
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		address, err := nodeSSHAddress(node)
		if err != nil {
			return fmt.Errorf("control-plane node %q: %w", node.Name, err)
		}

		klog.Infof("Writing cluster state to control-plane node %q (%s)", node.Name, address)
		if out != nil {
			fmt.Fprintf(out, "Writing cluster state to control-plane node %q (%s)\n", node.Name, address)
		}

		sshTarget, err := NewSSHHost(ctx, address, sshPort, sshUser, sudo)
		if err != nil {
			return fmt.Errorf("connecting to control-plane node %q (%s): %w", node.Name, address, err)
		}
		if err := sshTarget.WriteFilesToHost(ctx, files); err != nil {
			sshTarget.Close()
			return fmt.Errorf("writing files to control-plane node %q (%s): %w", node.Name, address, err)
		}
		if err := sshTarget.Close(); err != nil {
			return fmt.Errorf("closing SSH connection to control-plane node %q (%s): %w", node.Name, address, err)
		}
	}
	return nil
}

func restartKopsControllerPods(ctx context.Context, k8sClient kubernetes.Interface) error {
	if err := k8sClient.CoreV1().Pods("kube-system").DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "k8s-app=kops-controller",
	}); err != nil {
		return fmt.Errorf("restarting kops-controller: %w", err)
	}
	return nil
}

func patchKopsControllerConfigMap(ctx context.Context, k8sClient kubernetes.Interface, cluster *kops.Cluster) error {
	cm, err := k8sClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kops-controller", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting kops-controller ConfigMap: %w", err)
	}

	configYAML, ok := cm.Data["config.yaml"]
	if !ok {
		return fmt.Errorf("kops-controller ConfigMap has no config.yaml key")
	}

	opts := &kopscontrollerconfig.Options{}
	if err := json.Unmarshal([]byte(configYAML), opts); err != nil {
		return fmt.Errorf("parsing kops-controller config: %w", err)
	}

	configBase, secretStore := metal.KopsControllerStorePaths(cluster)
	opts.ConfigBase = configBase
	opts.SecretStore = secretStore

	updated, err := json.Marshal(opts)
	if err != nil {
		return fmt.Errorf("serializing kops-controller config: %w", err)
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["config.yaml"] = string(updated)

	if _, err := k8sClient.CoreV1().ConfigMaps("kube-system").Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating kops-controller ConfigMap: %w", err)
	}

	klog.Infof("patched kops-controller ConfigMap configBase=%q secretStore=%q", configBase, secretStore)
	return nil
}

func ensureKopsControllerStateStoreMount(ctx context.Context, k8sClient kubernetes.Interface) error {
	ds, err := k8sClient.AppsV1().DaemonSets("kube-system").Get(ctx, "kops-controller", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting kops-controller DaemonSet: %w", err)
	}

	for _, volume := range ds.Spec.Template.Spec.Volumes {
		if volume.Name == "kops-state-store" {
			return nil
		}
	}

	hostPathDirectory := corev1.HostPathDirectory
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: "kops-state-store",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: metal.LocalConfigRoot,
				Type: &hostPathDirectory,
			},
		},
	})

	if len(ds.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("kops-controller DaemonSet has no containers")
	}
	container := &ds.Spec.Template.Spec.Containers[0]
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "kops-state-store",
		MountPath: metal.LocalConfigRoot,
		ReadOnly:  true,
	})

	if _, err := k8sClient.AppsV1().DaemonSets("kube-system").Update(ctx, ds, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating kops-controller DaemonSet: %w", err)
	}
	return nil
}

func nodeSSHAddress(node *corev1.Node) (string, error) {
	for _, addressType := range []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP} {
		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == addressType && nodeAddress.Address != "" {
				return nodeAddress.Address, nil
			}
		}
	}
	return "", fmt.Errorf("node has no InternalIP or ExternalIP address")
}
