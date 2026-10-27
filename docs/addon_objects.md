# Passing additional configuration objects

kOps supports passing additional objects to the cluster, and recognizes certain "well known" objects.

Objects that are not well-known will be applied directly to the cluster's k8s api.

## Adding objects to a cluster

To add an additional object to an existing cluster, use `kops create -f` with the cluster name
(from `--name` or `KOPS_CLUSTER_NAME`):

```
kops create -f scheduler-config.yaml --name=my.cluster.k8s.local
kops update cluster --name=my.cluster.k8s.local --yes --admin
kops rolling-update cluster --name=my.cluster.k8s.local --yes
```

Objects are merged with the ones already stored for the cluster: an object with the same kind, namespace
and name is replaced, anything else is added.

When creating a new cluster, you can use the `--add` flag (requires the `ClusterAddons` feature flag):

```
export KOPS_FEATURE_FLAGS=ClusterAddons
kops create cluster --name=my.cluster.k8s.local --zones us-east-2a --add scheduler-config.yaml
kops update cluster --name=my.cluster.k8s.local --yes --admin
```

To view the current additional objects for a cluster:

```
kops get all -o yaml
```

# Well-Known Objects

Well-known objects receive special handling from kOps instead of being applied directly to the cluster.

## KubeSchedulerConfiguration (group: kubescheduler.config.k8s.io)

KubeSchedulerConfiguration objects allow for custom configuration of
kube-scheduler, the component responsible for assigning Pods to Nodes.

Fields set in `spec.kubeScheduler` (see [cluster_spec.md](cluster_spec.md#kubescheduler)) are merged on top of the KubeSchedulerConfiguration object. 
This allows you to use KubeSchedulerConfiguration for advanced settings like scheduler profiles and plugins, while using `spec.kubeScheduler` for simpler fields.

Note: the `clientConnection.kubeconfig` field is managed by kOps and must not be set manually.

Example KubeSchedulerConfiguration file:

```yaml
# scheduler-config.yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: default-scheduler
    plugins:
      score:
        disabled:
          - name: NodeResourcesBalancedAllocation
```

Example usage with an existing cluster:

```
kops create -f scheduler-config.yaml
kops update cluster --name=my.cluster.k8s.local --yes --admin
kops rolling-update cluster --name=my.cluster.k8s.local --yes
```

# Other Kubernetes Objects

Any Kubernetes object that is not a well-known type (such as ConfigMaps, Deployments,
DaemonSets, Services, RBAC resources, etc.) will be applied directly to the cluster
as a standard Kubernetes resource during `kops update cluster`.

# ClusterPackage and GCPResource (experimental)

These objects require the `ClusterPackages` feature flag:

```
export KOPS_FEATURE_FLAGS=ClusterPackages
```

## ClusterPackage (group: addons.kops.k8s.io)

A ClusterPackage installs a Kubernetes manifest as a kOps-managed addon. kOps fetches the manifest when
you run `kops update cluster`, stores it in the state store next to its own addons, and the
kops-channels agent on the control plane applies it and keeps it applied. This is the recommended way
to install software that ships a plain manifest, such as cert-manager.

```yaml
apiVersion: addons.kops.k8s.io/v1alpha1
kind: ClusterPackage
metadata:
  name: cert-manager
spec:
  version: v1.21.3
  manifest:
    url: https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.yaml
    sha256: 0123456789abcdef...   # optional, but recommended for https URLs
```

Fields:

* `spec.version` (required) identifies the package version. It is used as the addon id, so changing it
  makes kOps re-apply the manifest. Use it to upgrade: bump `version` and `manifest.url` together.
* `spec.manifest.url` is where the manifest is fetched from. `https://`, `gs://`, `s3://` and `file://`
  URLs are supported; plain `http://` is not. `spec.manifest.sha256` optionally pins the content.
* `spec.manifest.inline` embeds a small manifest directly instead of `url`.
* `spec.render` (default `false`) treats the manifest as a Go template rendered against a small set of
  cluster values (see below). Manifests are applied verbatim unless this is set.
* `spec.prune` (default `true`) deletes objects that an earlier version of the package created but the
  current manifest no longer contains.

Every object in the manifest is labelled `app.kubernetes.io/managed-by=kops` and
`addon.kops.k8s.io/name=<package name>`. The package name is also the `k8s-addon` selector label, so it
must be a valid label value.

Install or update a package with `kops create -f package.yaml` followed by `kops update cluster --yes`.
`kops create -f` merges with the objects already stored for the cluster; it does not replace them.

### Values available when `render: true`

| Value | Example |
|-------|---------|
| `{{ .ClusterName }}` | `gw.k8s.local` |
| `{{ .CloudProvider }}` | `gce` |
| `{{ .Region }}` | `us-central1` |
| `{{ .KubernetesVersion }}` | `1.37.1` |
| `{{ .GCP.Project }}` | `my-project` (GCE only) |
| `{{ .GCP.ControlPlaneServiceAccount }}` | `control-plane-gw-k8s-local@my-project.iam.gserviceaccount.com` (GCE only) |
| `{{ .GCP.NodeServiceAccount }}` | `node-gw-k8s-local@my-project.iam.gserviceaccount.com` (GCE only) |

Referencing a value that does not exist is an error at `kops update cluster` time. This set is kept
deliberately small so that it can stay stable; the template functions used by kOps' own addons are not
available to packages.

## GCPResource (group: addons.kops.k8s.io)

A GCPResource declares a Google Cloud resource that a package needs, for example an enabled API or an
IAM binding. The object's `metadata.name` is the resource URL in the form used by Cloud Asset Inventory,
and `spec` is the resource body in that API's own schema. Names and specs are rendered against the same
values as ClusterPackage manifests.

```yaml
apiVersion: addons.kops.k8s.io/v1alpha1
kind: GCPResource
metadata:
  name: //serviceusage.googleapis.com/projects/{{ .GCP.Project }}/services/dns.googleapis.com
spec:
  state: ENABLED
```

GCPResource objects are validated and parsed today, but creating the cloud resources is not implemented
yet: `kops update cluster` fails with a clear error if any are present. The object is documented now so
that packages can be written against the intended schema.
