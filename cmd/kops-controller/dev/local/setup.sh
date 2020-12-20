#!/bin/bash

set -o errexit -o nounset -o pipefail

# cd to the repo root
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

kubectl apply -f ${REPO_ROOT}/k8s/crds


kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_clusters.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinedeployments.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machines.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinesets.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinehealthchecks.yaml

kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpclusters.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpmachines.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpmachinetemplates.yaml

kubectl label crd gcpclusters.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3
kubectl label crd gcpmachines.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3
kubectl label crd gcpmachinetemplates.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3

kubectl label crd gcpclusters.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3
kubectl label crd gcpmachines.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3
kubectl label crd gcpmachinetemplates.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3

kubectl label crd clusters.kops.k8s.io cluster.x-k8s.io/v1alpha4=v1alpha2


kops get cluster foo.containuous.com -oyaml | kubectl apply -f -
kops get ig nodes-us-east4-a -oyaml | kubectl apply -f -

kubectl apply -f cmd/kops-controller/dev/local/cluster.yaml 
