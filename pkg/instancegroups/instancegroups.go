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

package instancegroups

import (
	"k8s.io/client-go/pkg/api/v1"
	api "k8s.io/kops/pkg/apis/kops"
)

type HasCloudInstanceGroups interface {
	FindCloudInstanceGroups(cluster *api.Cluster, instancegroups []*api.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*CloudInstanceGroup, error)
	DeleteCloudInstanceGroup(*CloudInstanceGroup) error
	DeleteCloudInstanceGroupMember(*CloudInstanceGroup, *CloudInstanceGroupInstance) error
}

// CloudInstanceGroup is the AWS ASG backing an InstanceGroup.
type CloudInstanceGroup struct {
	InstanceGroup *api.InstanceGroup
	Id            string
	MinSize       int
	MaxSize       int
	Status        string
	Ready         []*CloudInstanceGroupInstance
	NeedUpdate    []*CloudInstanceGroupInstance

	CloudObject interface{}
}

// CloudInstanceGroupInstance describes an instance in an autoscaling group.
type CloudInstanceGroupInstance struct {
	ID   string
	Node *v1.Node

	CloudObject interface{}
}

func (n *CloudInstanceGroup) String() string {
	return "CloudInstanceGroup:" + n.Id
}
