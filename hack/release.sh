#!/bin/bash

# Script used by cloudbuild

set -o errexit
set -o nounset
set -o pipefail

set -ex

WORKDIR=`pwd`
echo "WORKDIR=$WORKDIR"

apt-get update
apt-get install --yes make wget python

# Install gsutil
wget https://storage.googleapis.com/pub/gsutil.tar.gz
SHASUM=a809934d99b0647bb3940a0819c9f7705178547c03bf84082fa1c9fe73804af2
echo "${SHASUM} *gsutil.tar.gz" | sha256sum -c -
tar -C /usr/local -zxf gsutil.tar.gz
rm gsutil.tar.gz
export PATH="/usr/local/gsutil:$PATH"
gsutil version

# Install golang
wget -O go.tar.gz https://dl.google.com/go/go1.10.4.linux-amd64.tar.gz
SHASUM=fa04efdb17a275a0c6e137f969a1c4eb878939e91e1da16060ce42f02c2ec5ec
echo "${SHASUM} *go.tar.gz" | sha256sum -c -
tar -C /usr/local -zxf go.tar.gz
rm go.tar.gz
export PATH="/usr/local/go/bin:$PATH"
go version


make dns-controller-push DOCKER_REGISTRY=gcr.io/justinsb-cloud-kubernetes-test

make version-dist

mkdir ${WORKDIR}/artifacts
cp -r ${WORKDIR}/.build/upload/kops/* ${WORKDIR}/artifacts/

find ${WORKDIR}/artifacts/
