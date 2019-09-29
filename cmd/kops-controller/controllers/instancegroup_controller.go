/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kopsv1alpha2 "k8s.io/kops/cmd/kops-controller/api/v1alpha2"
)

// InstanceGroupReconciler reconciles a InstanceGroup object
type InstanceGroupReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=kops.kops.k8s.io,resources=instancegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kops.kops.k8s.io,resources=instancegroups/status,verbs=get;update;patch

func (r *InstanceGroupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.TODO()

	// Fetch the InstanceGroup instance
	instance := &api.InstanceGroup{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	cluster, err := r.getCluster(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	b := &clusterapi.Builder{
		//	ClientSet: todo,
	}
	md, err := b.BuildMachineDeployment(cluster, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateMachineDeployment(md); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ReconcileInstanceGroup) updateMachineDeployment(md *unstructured.Unstructured) error {
	gvr := schema.GroupVersionResource{
		Group:    "cluster.k8s.io",
		Version:  "v1alpha2",
		Resource: "machinedeployments",
	}
	namespace := md.GetNamespace()
	name := md.GetName()
	res := r.dynamicClient.Resource(gvr).Namespace(namespace)
	existing, err := res.Get(name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			existing = nil
		} else {
			return fmt.Errorf("error getting machindeployment %s/%s: %v", namespace, name, err)
		}
	}
	if existing == nil {
		var opts metav1.CreateOptions
		if _, err := res.Create(md, opts); err != nil {
			return fmt.Errorf("error creating machindeployment %s/%s: %v", namespace, name, err)
		}
		return nil
	}

	existing.Object["spec"] = md.Object["spec"]

	if _, err := res.Update(existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating machindeployment %s/%s: %v", namespace, name, err)
	}

	return nil
}

func (r *ReconcileInstanceGroup) getCluster(ctx context.Context, ig *api.InstanceGroup) (*api.Cluster, error) {
	clusters := &api.ClusterList{}
	var opts client.ListOptions
	opts.Namespace = ig.Namespace
	err := r.List(ctx, &opts, clusters)
	if err != nil {
		return nil, fmt.Errorf("error fetching clusters: %v", err)
	}

	if len(clusters.Items) == 0 {
		return nil, fmt.Errorf("cluster not found in namespace %q", ig.Namespace)
	}

	if len(clusters.Items) > 1 {
		return nil, fmt.Errorf("multiple clusters found in namespace %q", ig.Namespace)
	}

	return &clusters.Items[0], nil

}

func (r *InstanceGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kopsv1alpha2.InstanceGroup{}).
		Complete(r)
}
