### Changes from [upstream version](https://github.com/kubernetes/dashboard/tree/master/src/deploy/recommended):

* Add label `k8s-addon: kubernetes-dashboard.addons.k8s.io` to all top-level objects
* Rename `kubernetes-dashboard-minimal` rbac objects to `kubernetes-dashboard`
* Add critical pod annotations to deployment.spec.template.metadata.annotations:

```
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
        scheduler.alpha.kubernetes.io/tolerations: '[{"key":"CriticalAddonsOnly", "operator":"Exists"}]'
```


### Changes that have also been made upstream and should no longer be needed:

* Switch to _not_ use a NodePort; we access through the kube-api proxy instead
