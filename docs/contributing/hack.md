# Hack Directory

The `hack/` directory in the root of the repo holds scripts that make developing kOps easier. The ones you
will use most are the `dev-build-*.sh` scripts, which let you run a cluster from your working tree instead
of from a released binary.

## Running kOps from source with `dev-build-*.sh`

When you change kOps you usually need to test two binaries: `kops` (runs on your machine) and `nodeup`
(runs on every instance during boot, and is downloaded from `KOPS_BASE_URL`). The `dev-build-*.sh` scripts
build both, upload `nodeup` and the kOps container images to a per-developer bucket, and export the
environment variables that make `kops` use them:

| Script | Cloud | Needs |
|--------|-------|-------|
| `hack/dev-build-aws.sh` | AWS | `aws` CLI configured; uploads to `s3://kops-dev-<account>-<user>` |
| `hack/dev-build-gce.sh` | GCE | `gcloud` configured; uploads to `gs://kops-dev-<project>-<user>` |
| `hack/dev-build-scaleway.sh` | Scaleway | `scw` CLI configured |
| `hack/dev-build-metal.sh` | Bare metal | the local fake S3 server from `docs/metal.md` |

The scripts must be *sourced* so that the exported variables land in your shell:

```shell
. hack/dev-build-gce.sh
```

On success they print `SUCCESS` and your shell has `KOPS_BASE_URL`, `KOPS_STATE_STORE`, and `UPLOAD_DEST`
set. The `kops` binary is installed into `$(go env GOBIN)` (or `$(go env GOPATH)/bin`); check that this comes
before any released `kops` on your `PATH`, or call it with its full path. After you change code, re-source
the script to rebuild and re-upload. The variables only exist in the shell where you sourced the script.

### Targeting a particular GCP project

`dev-build-gce.sh` uses whatever `gcloud config get-value project` returns. To use a different project
without changing your global gcloud configuration, set `CLOUDSDK_CORE_PROJECT` before sourcing:

```shell
export CLOUDSDK_CORE_PROJECT=my-dev-project
. hack/dev-build-gce.sh
```

Pass the same project to `kops create cluster --project=...`.

### Letting GCE nodes read the dev bucket

The GCE script uses a `gs://` URL for `KOPS_BASE_URL` so that instances download `nodeup` with their own
service-account credentials. kOps automatically grants the cluster's service accounts read access to the
state store bucket, but not to the separate dev bucket, so after the first `kops update cluster --yes` for
a new cluster you need to grant it yourself (the service accounts are created by that command):

```shell
CLUSTER=simple-k8s-local      # cluster name with dots replaced by dashes
PROJECT=$(gcloud config get-value project)
for sa in control-plane-${CLUSTER} node-${CLUSTER}; do
  gcloud storage buckets add-iam-policy-binding "${UPLOAD_DEST}" \
    --member="serviceAccount:${sa}@${PROJECT}.iam.gserviceaccount.com" \
    --role=roles/storage.objectViewer
done
```

Instances that booted before the grant keep retrying the download, so the cluster recovers on its own
once the binding exists.

## Other useful scripts

- `hack/dev-push-kops-controller.sh` rebuilds the kops-controller image, pushes it, and bounces the pods in
  a running cluster, for iterating on the controller without a full rolling update.
- `hack/update-expected.sh` regenerates the golden files used by the integration tests after an intentional
  change to generated output.
- `hack/verify-*.sh` are the checks run by `make verify` in CI; run the relevant one locally before pushing.
