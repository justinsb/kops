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

package instancegroups

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/klog/v2"

	"k8s.io/kops/pkg/cloudinstances"
	"k8s.io/kops/pkg/commands"
)

func (c *RollingUpdateCluster) updateInPlace(ctx context.Context, u *cloudinstances.CloudInstance) error {
	node := u.Node
	if node == nil {
		return fmt.Errorf("cannot update instance %q in-place: instance is not registered in kubernetes", u.ID)
	}

	ig := u.CloudInstanceGroup.InstanceGroup

	klog.Infof("Updating instance %q, node %q, in group %q in-place (this may take a while).", u.ID, node.Name, u.CloudInstanceGroup.HumanName)

	bootstrapData, err := c.bootstrapDataForInstanceGroup(ctx, ig.ObjectMeta.Name)
	if err != nil {
		return fmt.Errorf("building bootstrap data for instance group %q: %w", ig.ObjectMeta.Name, err)
	}

	address := ""
	for _, addressType := range []corev1.NodeAddressType{corev1.NodeInternalIP, corev1.NodeExternalIP} {
		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == addressType && nodeAddress.Address != "" {
				address = nodeAddress.Address
				break
			}
		}
		if address != "" {
			break
		}
	}
	if address == "" {
		return fmt.Errorf("cannot update node %q in-place: node has no InternalIP or ExternalIP address", node.Name)
	}

	sshUser := c.Options.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}
	sshPort := c.Options.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	sudo := sshUser != "root"

	sshTarget, err := commands.NewSSHHost(ctx, address, sshPort, sshUser, sudo)
	if err != nil {
		return fmt.Errorf("connecting to node %q (%s): %w", node.Name, address, err)
	}
	defer sshTarget.Close()

	if err := sshTarget.ApplyNodeup(ctx, bootstrapData, commands.ApplyNodeupOptions{Wait: true}); err != nil {
		return fmt.Errorf("applying nodeup on node %q (%s): %w", node.Name, address, err)
	}

	return nil
}

// bootstrapDataForInstanceGroup builds (and caches) the bootstrap data used to run nodeup
// on machines in the given instance group.
func (c *RollingUpdateCluster) bootstrapDataForInstanceGroup(ctx context.Context, instanceGroupName string) (*commands.BootstrapData, error) {
	c.inPlaceBootstrapDataMutex.Lock()
	defer c.inPlaceBootstrapDataMutex.Unlock()

	if bootstrapData, found := c.inPlaceBootstrapData[instanceGroupName]; found {
		return bootstrapData, nil
	}

	configBuilder := &commands.ConfigBuilder{
		Clientset:         c.Clientset,
		Cloud:             c.Cloud,
		ClusterName:       c.Cluster.ObjectMeta.Name,
		InstanceGroupName: instanceGroupName,
	}

	bootstrapData, err := configBuilder.GetBootstrapData(ctx)
	if err != nil {
		return nil, err
	}

	if c.inPlaceBootstrapData == nil {
		c.inPlaceBootstrapData = make(map[string]*commands.BootstrapData)
	}
	c.inPlaceBootstrapData[instanceGroupName] = bootstrapData
	return bootstrapData, nil
}

// makeNodeSchedulable reverses the effects of draining, after an in-place update:
// it uncordons the node, removes the scheduled-for-update taint,
// and removes the exclude-from-load-balancers label.
// It also removes the needs-update annotation; on cloud providers that annotation
// disappears when the node is replaced, but here the node object lives on.
//
// After a control-plane in-place update the apiserver may be temporarily unreachable
// while static pods restart; retry until the cluster validates again.
func (c *RollingUpdateCluster) makeNodeSchedulable(ctx context.Context, nodeName string) error {
	timeout := c.ValidationTimeout
	if timeout == 0 {
		timeout = 15 * time.Minute
	}
	tick := c.ValidateTickDuration
	if tick == 0 {
		tick = 10 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		err := c.makeNodeSchedulableOnce(ctx, nodeName)
		if err == nil {
			return nil
		}
		if !isAPIServerUnreachable(err) || time.Now().After(deadline) {
			return err
		}
		klog.Infof("API not reachable while uncordoning node %q, will retry in %s: %v", nodeName, tick, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(tick):
		}
	}
}

func (c *RollingUpdateCluster) makeNodeSchedulableOnce(ctx context.Context, nodeName string) error {
	// Fetch a fresh copy of the node; it has likely been modified since we drained it.
	node, err := c.K8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("node %q not found after in-place update; skipping uncordon", nodeName)
			return nil
		}
		return fmt.Errorf("getting node %q: %w", nodeName, err)
	}

	oldData, err := json.Marshal(node)
	if err != nil {
		return err
	}

	node.Spec.Unschedulable = false

	var taints []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key == rollingUpdateTaintKey {
			continue
		}
		taints = append(taints, taint)
	}
	node.Spec.Taints = taints

	delete(node.Labels, corev1.LabelNodeExcludeBalancers)
	delete(node.Annotations, "kops.k8s.io/needs-update")

	newData, err := json.Marshal(node)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, node)
	if err != nil {
		return err
	}

	klog.Infof("Uncordoning node %q after in-place update.", nodeName)
	if _, err := c.K8sClient.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patching node %q: %w", nodeName, err)
	}
	return nil
}

func isAPIServerUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsServiceUnavailable(err) || apierrors.IsTimeout(err) || apierrors.IsTooManyRequests(err) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isAPIServerUnreachable(urlErr.Err)
	}
	return false
}
