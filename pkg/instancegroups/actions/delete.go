package actions

import (
	"fmt"

	"github.com/golang/glog"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/client/simple"
	"k8s.io/kops/pkg/instancegroups"
)

// DeleteInstanceGroup removes the cloud resources for an InstanceGroup
type DeleteInstanceGroup struct {
	Cluster   *api.Cluster
	Cloud     instancegroups.HasCloudInstanceGroups
	Clientset simple.Clientset
}

func (c *DeleteInstanceGroup) DeleteInstanceGroup(group *api.InstanceGroup) error {
	groups, err := c.Cloud.FindCloudInstanceGroups(c.Cluster, []*api.InstanceGroup{group}, false, nil)
	if err != nil {
		return fmt.Errorf("error finding CloudInstanceGroups: %v", err)
	}
	cig := groups[group.ObjectMeta.Name]
	if cig == nil {
		glog.Warningf("AutoScalingGroup %q not found in cloud - skipping delete", group.ObjectMeta.Name)
	} else {
		if len(groups) != 1 {
			return fmt.Errorf("Multiple InstanceGroup resources found in cloud")
		}

		glog.Infof("Deleting AutoScalingGroup %q", group.ObjectMeta.Name)

		err = c.Cloud.DeleteCloudInstanceGroup(cig)
		if err != nil {
			return fmt.Errorf("error deleting cloud resources for InstanceGroup: %v", err)
		}
	}

	err = c.Clientset.InstanceGroupsFor(c.Cluster).Delete(group.ObjectMeta.Name, nil)
	if err != nil {
		return err
	}

	return nil
}
