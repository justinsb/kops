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

package vfsclientset

import (
	"fmt"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/validation"
	"k8s.io/kops/pkg/k8scodecs"
)

type AddonsVFS struct {
	commonVFS

	clusterName string
	cluster     *kops.Cluster
}

func newAddonsVFS(c *VFSClientset, cluster *kops.Cluster) *AddonsVFS {
	if cluster == nil || cluster.Name == "" {
		glog.Fatalf("cluster / cluster.Name is required")
	}

	clusterName := cluster.Name
	kind := "ConfigMap"

	r := &AddonsVFS{
		cluster:     cluster,
		clusterName: clusterName,
	}
	r.init(kind, c.basePath.Join(clusterName, "addon"), StoreVersion)
	defaultReadVersion := v1.SchemeGroupVersion.WithKind(kind)
	r.defaultReadVersion = &defaultReadVersion
	r.validate = func(o runtime.Object) error {
		return validation.ValidateAddon(o.(*v1.ConfigMap))
	}

	codecs := k8scodecs.Codecs
	yaml, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), "application/yaml")
	if !ok {
		glog.Fatalf("no YAML serializer registered")
	}
	r.encoder = codecs.EncoderForVersion(yaml.Serializer, v1.SchemeGroupVersion)
	r.decoder = codecs.DecoderToVersion(yaml.Serializer, v1.SchemeGroupVersion)

	return r
}

var _ corev1client.ConfigMapInterface = &AddonsVFS{}

func (v *AddonsVFS) Get(name string, options metav1.GetOptions) (*v1.ConfigMap, error) {
	if options.ResourceVersion != "" {
		return nil, fmt.Errorf("ResourceVersion not supported in AddonsVFS::Get")
	}

	o, err := v.find(name)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errors.NewNotFound(schema.GroupResource{Group: v1.GroupName, Resource: "ConfigMap"}, name)
	}
	cm := o.(*v1.ConfigMap)
	v.addLabels(cm)

	return cm, nil
}

func (v *AddonsVFS) addLabels(o *v1.ConfigMap) {
	// if ig.ObjectMeta.Labels == nil {
	// 	ig.ObjectMeta.Labels = make(map[string]string)
	// }
	// ig.ObjectMeta.Labels[api.LabelClusterName] = c.clusterName
}

func (v *AddonsVFS) List(options metav1.ListOptions) (*v1.ConfigMapList, error) {
	list := &v1.ConfigMapList{}
	items, err := v.list(list.Items, options)
	if err != nil {
		return nil, err
	}
	list.Items = items.([]v1.ConfigMap)
	for i := range list.Items {
		v.addLabels(&list.Items[i])
	}
	return list, nil
}

func (v *AddonsVFS) Create(o *v1.ConfigMap) (*v1.ConfigMap, error) {
	err := v.create(v.cluster, o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (v *AddonsVFS) Update(o *v1.ConfigMap) (*v1.ConfigMap, error) {
	err := v.update(v.cluster, o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (v *AddonsVFS) Delete(name string, options *metav1.DeleteOptions) error {
	return v.delete(name, options)
}

func (v *AddonsVFS) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("AddonsVFS DeleteCollection not implemented for vfs store")
}

func (v *AddonsVFS) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("AddonsVFS Watch not implemented for vfs store")
}

func (v *AddonsVFS) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ConfigMap, err error) {
	return nil, fmt.Errorf("AddonsVFS Patch not implemented for vfs store")
}
