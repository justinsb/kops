To use CRDs:

# Apply our CRDs
kubectl apply -f config/crds/

# Install cluster-api
kubectl apply -f https://github.com/kubernetes-sigs/cluster-api/releases/download/v0.2.3/cluster-api-components.yaml

# Register a CRD instancegroup derived from the nodes ig
kops get instancegroup nodes -oyaml | sed  -e s@nodes@morenodes@g | kubectl apply -f -
kops get cluster -oyaml | kubectl apply -f -

# TODO: Doesn't work ... we need to upload to S3/GCS
kops get instancegroup nodes -oyaml | kubectl apply -f -


# Install GCP provider
k apply -f examples/_out/provider-components.yaml

Manually remove GOOGLE_APPLICATION_CREDENTIALS env var

# Remove CAPBK
k delete crd kubeadmconfigs.bootstrap.cluster.x-k8s.io
k delete crd kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io
k delete ns cabpk-system

# TODO: Do we need a cluster
# examples/_out/cluster.yaml 

cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: GCPCluster
metadata:
  name: test-k8s-local
  namespace: default
spec:
  project: justinsb-cloud-kubernetes-test
  region: us-east4
---
apiVersion: cluster.x-k8s.io/v1alpha2
kind: Cluster
metadata:
  name: test-k8s-local
  namespace: default
spec:
  #clusterNetwork:
  #  pods:
  #    cidrBlocks:
  #    - 192.168.0.0/16
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
    kind: GCPCluster
    name: test-k8s-local
    namespace: default
EOF

# TODO: We need to normalize GCP resource names

# TODO: Why is project required?
# TODO: Why is region required?

cat <<EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
kind: GCPMachineTemplate
metadata:
  name: morenodes
  namespace: default
spec:
  template:
    spec:
      instanceType: n1-standard-2
      zone: us-east4-c
      image: cos-cloud/cos-stable-65-10323-99-0
---
apiVersion: cluster.x-k8s.io/v1alpha2
kind: MachineDeployment
metadata:
  labels:
    cluster.x-k8s.io/cluster-name: test-k8s-local
    kops.k8s.io/instancegroup: morenodes
  name: morenodes
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: test-k8s-local
      kops.k8s.io/instancegroup: morenodes
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: test-k8s-local
        kops.k8s.io/instancegroup: morenodes
    spec:
      bootstrap: {}
      #  configRef:
      #    apiVersion: bootstrap.cluster.x-k8s.io/v1alpha2
      #    kind: KubeadmConfigTemplate
      #    name: test1-md-0
      #    namespace: default
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha2
        kind: GCPMachineTemplate
        name: morenodes
        namespace: default
      #version: v1.16.0
EOF

# TODO: We should create the objects in the right order!!

# TODO: Why can't we qualify the image name?
