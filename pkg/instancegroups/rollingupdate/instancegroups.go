/*
Copyright 2016 The Kubernetes Authors.

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

package rollingupdate

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/featureflag"
	"k8s.io/kops/pkg/instancegroups"
	"k8s.io/kops/pkg/validation"
	"k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// TODO: Temporarily increase size of ASG?
// TODO: Remove from ASG first so status is immediately updated?
// TODO: Batch termination, like a rolling-update

// RollingUpdate performs a rolling update on a list of ec2 instances.
func (r *RollingUpdateCluster) RollingUpdateCloudInstanceGroup(n *instancegroups.CloudInstanceGroup, instanceGroupList *api.InstanceGroupList, isBastion bool, t time.Duration) (err error) {

	// we should not get here, but hey I am going to check.
	if r == nil {
		return fmt.Errorf("rollingUpdate cannot be nil")
	}

	// Do not need a k8s client if you are doing cloudonly.
	if r.K8sClient == nil && !r.CloudOnly {
		return fmt.Errorf("rollingUpdate is missing a k8s client")
	}

	if instanceGroupList == nil {
		return fmt.Errorf("rollingUpdate is missing the InstanceGroupList")
	}

	c := r.Cloud

	update := n.NeedUpdate
	if r.Force {
		update = append(update, n.Ready...)
	}

	if len(update) == 0 {
		return nil
	}

	if isBastion {
		glog.V(3).Info("Not validating the cluster as instance is a bastion.")
	} else if r.CloudOnly {
		glog.V(3).Info("Not validating cluster as validation is turned off via the cloud-only flag.")
	} else if featureflag.DrainAndValidateRollingUpdate.Enabled() {
		if err = r.ValidateCluster(instanceGroupList); err != nil {
			if r.FailOnValidate {
				return fmt.Errorf("error validating cluster: %v", err)
			} else {
				glog.V(2).Infof("Ignoring cluster validation error: %v", err)
				glog.Infof("Cluster validation failed, but proceeding since fail-on-validate-error is set to false")
			}
		}
	}

	for _, u := range update {
		instanceId := u.ID

		nodeName := ""
		if u.Node != nil {
			nodeName = u.Node.Name
		}

		if isBastion {

			if err = c.DeleteCloudInstanceGroupMember(n, u); err != nil {
				glog.Errorf("Error deleting instance %q: %v", instanceId, err)
				return err
			}

			glog.Infof("Deleted a bastion instance, %s, and continuing with rolling-update.", instanceId)

			continue

		} else if r.CloudOnly {

			glog.Warningf("Not draining cluster nodes as 'cloudonly' flag is set.")

		} else if featureflag.DrainAndValidateRollingUpdate.Enabled() {

			if u.Node != nil {
				glog.Infof("Draining the node: %q.", nodeName)

				if err = r.DrainNode(u); err != nil {
					if r.FailOnDrainError {
						return fmt.Errorf("Failed to drain node %q: %v", nodeName, err)
					} else {
						glog.Infof("Ignoring error draining node %q: %v", nodeName, err)
					}
				}
			} else {
				glog.Warningf("Skipping drain of instance %q, because it is not registered in kubernetes", instanceId)
			}
		}

		if err = c.DeleteCloudInstanceGroupMember(n, u); err != nil {
			glog.Errorf("Error deleting instance %q, node %q: %v", instanceId, nodeName, err)
			return err
		}

		// Wait for new EC2 instances to be created
		time.Sleep(t)

		if r.CloudOnly {

			glog.Warningf("Not validating cluster as cloudonly flag is set.")
			continue

		} else if featureflag.DrainAndValidateRollingUpdate.Enabled() {

			glog.Infof("Validating the cluster.")

			if err = r.ValidateClusterWithDuration(instanceGroupList, t); err != nil {

				if r.FailOnValidate {
					glog.Errorf("Cluster did not validate within the set duration of %q, you can retry, and maybe extend the duration", t)
					return fmt.Errorf("error validating cluster after removing a node: %v", err)
				}

				glog.Warningf("Cluster validation failed after removing instance, proceeding since fail-on-validate is set to false: %v", err)
			}
		}
	}

	return nil
}

// ValidateClusterWithDuration runs validation.ValidateCluster until either we get positive result or the timeout expires
func (r *RollingUpdateCluster) ValidateClusterWithDuration(instanceGroupList *api.InstanceGroupList, duration time.Duration) error {
	// TODO should we expose this to the UI?
	tickDuration := 30 * time.Second
	// Try to validate cluster at least once, this will handle durations that are lower
	// than our tick time
	if r.tryValidateCluster(instanceGroupList, duration, tickDuration) {
		return nil
	}

	timeout := time.After(duration)
	tick := time.Tick(tickDuration)
	// Keep trying until we're timed out or got a result or got an error
	for {
		select {
		case <-timeout:
			// Got a timeout fail with a timeout error
			return fmt.Errorf("cluster did not validate within a duation of %q", duration)
		case <-tick:
			// Got a tick, validate cluster
			if r.tryValidateCluster(instanceGroupList, duration, tickDuration) {
				return nil
			}
			// ValidateCluster didn't work yet, so let's try again
			// this will exit up to the for loop
		}
	}
}

func (r *RollingUpdateCluster) tryValidateCluster(instanceGroupList *api.InstanceGroupList, duration time.Duration, tickDuration time.Duration) bool {
	if _, err := validation.ValidateCluster(r.ClusterName, instanceGroupList, r.K8sClient); err != nil {
		glog.Infof("Cluster did not validate, will try again in %q util duration %q expires: %v.", tickDuration, duration, err)
		return false
	} else {
		glog.Infof("Cluster validated.")
		return true
	}
}

// ValidateCluster runs our validation methods on the K8s Cluster.
func (r *RollingUpdateCluster) ValidateCluster(instanceGroupList *api.InstanceGroupList) error {
	if _, err := validation.ValidateCluster(r.ClusterName, instanceGroupList, r.K8sClient); err != nil {
		return fmt.Errorf("cluster %q did not pass validation: %v", r.ClusterName, err)
	}

	return nil

}

// DrainNode drains a K8s node.
func (r *RollingUpdateCluster) DrainNode(u *instancegroups.CloudInstanceGroupInstance) error {
	if r.ClientConfig == nil {
		return fmt.Errorf("ClientConfig not set")
	}
	f := cmdutil.NewFactory(r.ClientConfig)

	// TODO: Send out somewhere else, also DrainOptions has errout
	out := os.Stdout
	errOut := os.Stderr

	options := &cmd.DrainOptions{
		Factory:          f,
		Out:              out,
		IgnoreDaemonsets: true,
		Force:            true,
		DeleteLocalData:  true,
		ErrOut:           errOut,
	}

	cmd := &cobra.Command{
		Use: "cordon NODE",
	}
	args := []string{u.Node.Name}
	err := options.SetupDrain(cmd, args)
	if err != nil {
		return fmt.Errorf("error setting up drain: %v", err)
	}

	err = options.RunCordonOrUncordon(true)
	if err != nil {
		return fmt.Errorf("error cordoning node node: %v", err)
	}

	err = options.RunDrain()
	if err != nil {
		return fmt.Errorf("error draining node: %v", err)
	}

	if r.DrainInterval > time.Second*0 {
		glog.V(3).Infof("Waiting for %s for pods to stabilize after draining.", r.DrainInterval)
		time.Sleep(r.DrainInterval)
	}

	return nil
}
