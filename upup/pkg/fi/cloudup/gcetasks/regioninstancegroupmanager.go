///*
//Copyright 2016 The Kubernetes Authors.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//*/
//
package gcetasks

//
//import (
//	"fmt"
//	"github.com/golang/glog"
//	compute "google.golang.org/api/compute/v0.beta"
//	"k8s.io/kops/upup/pkg/fi"
//	"k8s.io/kops/upup/pkg/fi/cloudup/gce"
//	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
//	"time"
//)
//
////go:generate fitask -type=RegionInstanceGroupManager
//type RegionInstanceGroupManager struct {
//	Name *string
//
//	Zone             *string
//	BaseInstanceName *string
//	InstanceTemplate *InstanceTemplate
//	TargetSize       *int64
//
//	TargetPools []*TargetPool
//}
//
//var _ fi.CompareWithID = &RegionInstanceGroupManager{}
//
//func (e *RegionInstanceGroupManager) CompareWithID() *string {
//	return e.Name
//}
//
//func (e *RegionInstanceGroupManager) Find(c *fi.Context) (*RegionInstanceGroupManager, error) {
//	cloud := c.Cloud.(*gce.GCECloud)
//
//	r, err := cloud.Compute.RegionInstanceGroupManagers.Get(cloud.Project, *e.Zone, *e.Name).Do()
//	if err != nil {
//		if gce.IsNotFound(err) {
//			return nil, nil
//		}
//		return nil, fmt.Errorf("error listing RegionInstanceGroupManagers: %v", err)
//	}
//
//	actual := &RegionInstanceGroupManager{}
//	actual.Name = &r.Name
//	actual.Zone = fi.String(lastComponent(r.Zone))
//	actual.BaseInstanceName = &r.BaseInstanceName
//	actual.TargetSize = &r.TargetSize
//	actual.InstanceTemplate = &InstanceTemplate{Name: fi.String(lastComponent(r.InstanceTemplate))}
//
//	for _, targetPool := range r.TargetPools {
//		actual.TargetPools = append(actual.TargetPools, &TargetPool{
//			Name: fi.String(lastComponent(targetPool)),
//		})
//	}
//	// TODO: Sort by name
//
//	return actual, nil
//}
//
//func (e *RegionInstanceGroupManager) Run(c *fi.Context) error {
//	return fi.DefaultDeltaRunMethod(e, c)
//}
//
//func (_ *RegionInstanceGroupManager) CheckChanges(a, e, changes *RegionInstanceGroupManager) error {
//	return nil
//}
//func (_ *RegionInstanceGroupManager) RenderGCE(t *gce.GCEAPITarget, a, e, changes *RegionInstanceGroupManager) error {
//	project := t.Cloud.Project
//
//	i := &compute.InstanceGroupManager{
//		Name:             *e.Name,
//		Zone:             *e.Zone,
//		BaseInstanceName: *e.BaseInstanceName,
//		TargetSize:       *e.TargetSize,
//		InstanceTemplate: BuildInstanceTemplateURL(project, *e.InstanceTemplate.Name),
//	}
//
//	for _, targetPool := range e.TargetPools {
//		i.TargetPools = append(i.TargetPools, targetPool.URL(t.Cloud))
//	}
//
//	if a == nil {
//		for {
//			_, err := t.Cloud.Compute.RegionInstanceGroupManager.Insert(t.Cloud.Project, *e.Zone, i).Do()
//			if err != nil {
//				if gce.IsNotReady(err) {
//					glog.Infof("Found resourceNotReady error - sleeping before retry: %v", err)
//					time.Sleep(5 * time.Second)
//					continue
//				}
//				return fmt.Errorf("error creating RegionInstanceGroupManager: %v", err)
//			} else {
//				break
//			}
//		}
//	} else {
//		return fmt.Errorf("Cannot apply changes to RegionInstanceGroupManager: %v", changes)
//	}
//
//	return nil
//}
//
//func (_ *RegionInstanceGroupManager) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *RegionInstanceGroupManager) error {
//	return fmt.Errorf("Terraform does not support RegionInstanceGroupManager")
//}
