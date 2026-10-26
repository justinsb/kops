# Getting Started with kOps on GCE

Make sure you have [installed kOps](install.md) and [installed kubectl](install.md), and installed
the [gcloud CLI](https://cloud.google.com/sdk/docs/install).

## Prerequisites

You'll need a Google Cloud project with billing enabled. Make sure that gcloud is logged in and
pointing at that project, using `gcloud init` (or `gcloud config set project <project-id>`).

You should confirm that basic commands like `gcloud compute zones list` are working.

kOps talks to Google Cloud using
[Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials),
so you also need to run:

```shell
gcloud auth application-default login
```

Finally, enable the APIs that kOps uses. A brand-new project typically only has Cloud Storage enabled:

```shell
gcloud services enable compute.googleapis.com iam.googleapis.com cloudresourcemanager.googleapis.com
```

If you plan to use a real DNS zone for the cluster (`--dns public` or `--dns private`), also enable
`dns.googleapis.com`. The default cluster does not need it.

You do not need an SSH key to create a cluster on GCE. See [Accessing your nodes](#accessing-your-nodes)
below for how to log in to instances.

## Creating a state store

kOps needs a state store to hold the configuration for your clusters. The simplest configuration
for Google Cloud is to store it in a Google Cloud Storage bucket in the same project, so that's how we'll
start.

Create an empty bucket. Bucket names are globally unique, so including your project ID is a good way
to pick an available name:

```shell
PROJECT=$(gcloud config get-value project)
gcloud storage buckets create gs://kops-state-${PROJECT}/
```

Rather than typing the `--state` argument every time, it's much easier to export the `KOPS_STATE_STORE`
environment variable:

```shell
export KOPS_STATE_STORE=gs://kops-state-${PROJECT}/
```

You can also put this in your `~/.bashrc` or similar.

## Creating our first cluster

`kops create cluster` creates the Cluster object and InstanceGroup objects you'll be working with in kOps:

```shell
kops create cluster simple.k8s.local --zones us-central1-a --project=${PROJECT}
```

The cluster name has to look like a DNS name, but by default kOps does not create any DNS records
(`topology.dns.type: None`), so you can use any name you like. The `.k8s.local` suffix is a convention
for clusters that are not tied to a real domain.

This writes the configuration to the state store and prints a preview of the cloud resources that will be
created, but it doesn't create any instances yet.

You can now list the Cluster objects in your kOps state store (the GCS bucket we created):

```
> kops get cluster

NAME			CLOUD	ZONES
simple.k8s.local	gce	us-central1
```

This shows that you have one Cluster object configured, named `simple.k8s.local`. The cluster holds the
cluster-wide configuration for a kubernetes cluster - things like the kubernetes version, the networking
provider and the authorization policy in use.

The `kops` tool should feel a lot like `kubectl` - kOps uses the same API machinery as kubernetes,
so it should behave similarly, although now you are managing kubernetes clusters, instead of managing
objects on a kubernetes cluster.

You can see the details of your Cluster object by doing:

`kops get cluster simple.k8s.local -oyaml`

```yaml
apiVersion: kops.k8s.io/v1alpha2
kind: Cluster
metadata:
  name: simple.k8s.local
spec:
  api:
    loadBalancer:
      type: Public
  authorization:
    rbac: {}
  channel: stable
  cloudProvider: gce
  configBase: gs://kops-state-my-gce-project/simple.k8s.local
  etcdClusters:
  - etcdMembers:
    - instanceGroup: control-plane-us-central1-a
      name: a
    name: main
  - etcdMembers:
    - instanceGroup: control-plane-us-central1-a
      name: a
    name: events
  iam:
    allowContainerRegistry: true
    legacy: false
  kubernetesApiAccess:
  - 0.0.0.0/0
  - ::/0
  kubernetesVersion: 1.37.1
  networking:
    cilium:
      enableNodePort: true
  nonMasqueradeCIDR: 100.64.0.0/10
  project: my-gce-project
  sshAccess:
  - 0.0.0.0/0
  - ::/0
  subnets:
  - cidr: 10.0.16.0/20
    name: us-central1
    region: us-central1
    type: Public
  topology:
    dns:
      type: None
```

(Some defaulted fields have been trimmed from this output.)

Similarly, you can also see your InstanceGroups using:

```
> kops get instancegroup --name simple.k8s.local

NAME				ROLE		MACHINETYPE	MIN	MAX	ZONES
control-plane-us-central1-a	ControlPlane	e2-medium	1	1	us-central1-a
nodes-us-central1-a		Node		e2-medium	1	1	us-central1-a
```

InstanceGroups are the other main kOps object - an InstanceGroup manages a set of cloud instances,
which then are registered in kubernetes as Nodes. You have multiple InstanceGroups for different types
of instances / Nodes - in our simple example we have one for our control plane (which only has a single
member), and one for our worker nodes. Note that on GCE, subnets are regional (`us-central1`) while
InstanceGroups are zonal (`us-central1-a`).

We'll see a lot more of Cluster objects and InstanceGroups as we use kOps to reconfigure clusters. But let's get
on with our first cluster.

## Creating the cloud resources

`kops create cluster` created the Cluster object & InstanceGroup objects in our state store,
but didn't actually create any instances or other cloud objects in GCE. To do that, we'll use
`kops update cluster`.

`kops update cluster` without `--yes` shows you a preview of all the changes that will be made;
it is very useful to see what kOps is about to do, before actually making the changes.

Run `kops update cluster simple.k8s.local` and peruse the changes.

We're now finally ready to create the cluster:

```shell
kops update cluster simple.k8s.local --yes --admin
```

The `--admin` flag exports a time-limited admin credential (18 hours by default) for the new cluster into your kubeconfig and
switches your kubectl context to it. If you omit it, you can get the same result later with
`kops export kubeconfig simple.k8s.local --admin`.

Your cluster is created in the background - kOps creates GCE Managed Instance Groups
that run the instances; this ensures that even if instances are terminated, they will automatically
be relaunched by GCE and your cluster will self-heal.

It takes a few minutes for the instances to boot, download their configuration and start kubernetes.
`kops validate cluster` checks that the nodes have registered and the system pods are healthy, and
with `--wait` it keeps checking until the cluster is ready (or the timeout expires):

```shell
kops validate cluster --wait 10m
```

Once validation succeeds, `kubectl get nodes` shows your new cluster:

```
> kubectl get nodes

NAME                               STATUS   ROLES           AGE     VERSION
control-plane-us-central1-a-8wk7   Ready    control-plane   3m25s   v1.37.1
nodes-us-central1-a-cmgd           Ready    node            2m12s   v1.37.1
```

## Accessing your nodes

Because kOps uses GCE Managed Instance Groups, the easiest way to log in to a node is with
`gcloud compute ssh`, which manages SSH keys for you through the GCE metadata server:

```shell
gcloud compute instances list
gcloud compute ssh control-plane-us-central1-a-8wk7 --zone us-central1-a
```

If you prefer to use your own key, pass `--ssh-public-key ~/.ssh/id_ed25519.pub` to
`kops create cluster` (or add it later with `kops create sshpublickey`). kOps will install it for the
`ubuntu` user on every node.

## Enjoy

At this point you have a kubernetes cluster - the core commands to do so are as simple as `kops create cluster`
and `kops update cluster`. There's a lot more power in kOps, and even more power in kubernetes itself, so we've
put a few jumping off places here. But when you're done, don't forget to [delete your cluster](#deleting-the-cluster).

* [Manipulate InstanceGroups](../tutorial/working-with-instancegroups.md) to add more nodes, change image
* [Upgrade kubernetes](../operations/updates_and_upgrades.md) with `kops upgrade cluster` and `kops rolling-update cluster`
* [Install addons](../addons.md)

## Deleting the cluster

When you're done using the cluster, you should delete it to release the cloud resources. `kops delete cluster` is
the command. When run without `--yes` it shows a preview of the objects it will delete:

```
> kops delete cluster simple.k8s.local

TYPE			NAME							ID
Address			api-simple-k8s-local					api-simple-k8s-local
BackendService		api-simple-k8s-local					api-simple-k8s-local
Disk			a-etcd-events-simple-k8s-local				a-etcd-events-simple-k8s-local
Disk			a-etcd-main-simple-k8s-local				a-etcd-main-simple-k8s-local
FirewallRule		https-api-simple-k8s-local				https-api-simple-k8s-local
FirewallRule		node-to-master-simple-k8s-local				node-to-master-simple-k8s-local
...
ForwardingRule		api-simple-k8s-local					api-simple-k8s-local
Instance		control-plane-us-central1-a-8wk7			us-central1-a/control-plane-us-central1-a-8wk7
Instance		nodes-us-central1-a-cmgd				us-central1-a/nodes-us-central1-a-cmgd
InstanceGroupManager	a-control-plane-us-central1-a-simple-k8s-local		us-central1-a/a-control-plane-us-central1-a-simple-k8s-local
InstanceGroupManager	a-nodes-us-central1-a-simple-k8s-local			us-central1-a/a-nodes-us-central1-a-simple-k8s-local
InstanceTemplate	control-plane-us-central1-uoo9v5-1793037719		control-plane-us-central1-uoo9v5-1793037719
InstanceTemplate	nodes-us-central1-a-simpl-4fqdpv-1793037719		nodes-us-central1-a-simpl-4fqdpv-1793037719
Network			simple-k8s-local					simple-k8s-local
ServiceAccount		control-plane-simple-k8s-local@my-gce-project.iam.gserviceaccount.com	...
ServiceAccount		node-simple-k8s-local@my-gce-project.iam.gserviceaccount.com		...
Subnet			us-central1-simple-k8s-local				us-central1-simple-k8s-local
TargetPool		api-simple-k8s-local					api-simple-k8s-local

Must specify --yes to delete cluster
```

After you've double-checked you're deleting exactly what you want to delete, run
`kops delete cluster simple.k8s.local --yes`.

## Troubleshooting

### `kops create cluster` or `kops update cluster` hangs, logging `SERVICE_DISABLED` / `accessNotConfigured`

One of the required Google APIs is not enabled on the project. kOps retries the failing task for several
minutes before giving up. Enable the APIs listed under [Prerequisites](#prerequisites) and run the
command again.

### Every command fails with `403 UserProjectAccountProblem: User project billing account not in good standing`

Your Application Default Credentials carry a *quota project* (see the `quota_project_id` field in
`~/.config/gcloud/application_default_credentials.json`), and that project no longer has billing enabled.
gcloud itself keeps working because it uses different credentials, which makes this confusing. Point the
quota project at your current project:

```shell
gcloud auth application-default set-quota-project ${PROJECT}
```

or, for a single shell, `export GOOGLE_CLOUD_QUOTA_PROJECT=${PROJECT}`.

### The nodes never become Ready

Look at the serial console of a node to see what nodeup (the kOps bootstrap process) is doing:

```shell
gcloud compute instances get-serial-port-output control-plane-us-central1-a-8wk7 --zone us-central1-a | grep nodeup
```

Then see the general [troubleshooting guide](../operations/troubleshoot.md).

## Best practices

Below are some of the best practices when using kOps to create and administer a Kubernetes cluster on GCP.

### Use spot instances to reduce cost

[Spot instances](https://cloud.google.com/spot-vms) have the same specs as regular compute instances, but can be preempted at any time by higher priority instances. Using spot instances can reduce your compute cost by up to ~90%, so if your workloads are fault-tolerant this strategy can be extremely beneficial. Note that GCE handles preemption gracefully, giving you 30 seconds to shut down so you can safely checkpoint state/progress to be resumed later.

To create an instance group of spot instances, create your cluster as documented above, then update your instance group to use spot instances by performing the following steps:

- Run `kops edit ig <instance-group-name>` to edit the instance group config.
- Add the key-value pair `gcpProvisioningModel: SPOT` in the instance group `spec`:

```yaml
spec:
  gcpProvisioningModel: SPOT
```

- Run `kops update cluster --yes` followed by `kops rolling-update cluster --yes` to update the instance group.
- You can verify this succeeded on the [Google Cloud Platform developer console](https://console.cloud.google.com/) by navigating to Compute Engine, clicking on your particular node instance (by default it will be named something like `nodes-<zone>-<id>`) to pull up instance details, then under Management > Availability Policy there should be a setting that says `VM Provisioning Model: Spot`.

### Use a multi-zonal cluster for high availability

By default, kOps will create a cluster in a single [zone](https://cloud.google.com/compute/docs/regions-zones). In the event of an issue affecting
that particular datacenter (or even the particular server rack your VM instance is running on), this can cause availability issues for your cluster. The recommended solution is to use a **multi-zonal** cluster.

Specifically, it is recommended that both the control plane nodes and the worker nodes run in **3+ zones within a region**,
in order to ensure availability of both the control plane and worker nodes in the event of datacenter outages.

Example of creating a high availability multi-zonal cluster, with both the control plane and nodes running in 3 zones within a region:

```shell
kops create cluster \
  --control-plane-zones=us-central1-a,us-central1-b,us-central1-c \
  --zones=us-central1-a,us-central1-b,us-central1-c \
  --node-count=3 \
  --project=${PROJECT} \
  ${CLUSTER_NAME}
```

## Next steps

Now that you have a working kOps cluster, read through the [recommendations for production setups guide](production.md) to learn more about how to configure kOps for production workloads.
