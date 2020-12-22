#!/bin/bash

set -o errexit -o nounset -o pipefail

# cd to the repo root
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

kubectl apply -f ${REPO_ROOT}/k8s/crds


kubectl apply -f ~/k8s/src/sigs.k8s.io/cluster-api/config/crd/bases/
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_clusters.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinedeployments.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machines.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinesets.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/master/config/crd/bases/cluster.x-k8s.io_machinehealthchecks.yaml

kubectl apply -f ~/k8s/src/sigs.k8s.io/cluster-api-provider-gcp/config/crd/bases/
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpclusters.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpmachines.yaml
#kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-gcp/master/config/crd/bases/infrastructure.cluster.x-k8s.io_gcpmachinetemplates.yaml

kubectl label crd --overwrite gcpclusters.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3
kubectl label crd --overwrite gcpmachines.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3
kubectl label crd --overwrite gcpmachinetemplates.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha3=v1alpha3

kubectl label crd --overwrite gcpclusters.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3
kubectl label crd --overwrite gcpmachines.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3
kubectl label crd --overwrite gcpmachinetemplates.infrastructure.cluster.x-k8s.io cluster.x-k8s.io/v1alpha4=v1alpha3

kubectl label crd --overwrite clusters.kops.k8s.io cluster.x-k8s.io/v1alpha4=v1alpha2


CLUSTER=`kubectl config view --minify -ojson | jq -r '."current-context"'`
kops get cluster ${CLUSTER} -oyaml | kubectl apply -f -
kops get ig nodes-us-east4-a -oyaml | kubectl apply -f -

kubectl apply -f cmd/kops-controller/dev/local/cluster.yaml 
