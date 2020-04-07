## Building from source

To use S3:
```
# cd to your kops repo
export S3_BUCKET_NAME=kops-dev-${USER}
make kops-install dev-upload UPLOAD_DEST=s3://${S3_BUCKET_NAME}

KOPS_VERSION=`bazel run //cmd/kops version -- --short`
export KOPS_BASE_URL=https://${S3_BUCKET_NAME}.s3.amazonaws.com/kops/${KOPS_VERSION}/
```

To use GCS:
```
export GCS_BUCKET_NAME=kops-dev-${USER}
make kops-install dev-upload UPLOAD_DEST=gs://${GCS_BUCKET_NAME}

KOPS_VERSION=`bazel run //cmd/kops version -- --short`
export KOPS_BASE_URL=https://${GCS_BUCKET_NAME}.storage.googleapis.com/kops/${KOPS_VERSION}/
```

Whether using GCS or S3, you probably want to upload dns-controller &
kops-contoller images if you have changed them:

For dns-controller:

```bash
KOPS_VERSION=`bazel run //cmd/kops version -- --short`
export DOCKER_IMAGE_PREFIX=${USER}/
export DOCKER_REGISTRY=
make dns-controller-push
export DNSCONTROLLER_IMAGE=${DOCKER_IMAGE_PREFIX}dns-controller:${KOPS_VERSION}
```

For kops-controller:

```bash
KOPS_VERSION=`bazel run //cmd/kops version -- --short`
export DOCKER_IMAGE_PREFIX=${USER}/
export DOCKER_REGISTRY=
make kops-controller-push
export KOPSCONTROLLER_IMAGE=${DOCKER_IMAGE_PREFIX}kops-controller:${KOPS_VERSION}
```

You can create a cluster using `kops create cluster <clustername> --zones us-east-1b`
