/*
Copyright 2020 The Kubernetes Authors.

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

package deployer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"k8s.io/klog/v2"
	"k8s.io/kops/tests/e2e/kubetest2-kops/aws"
	"k8s.io/kops/tests/e2e/kubetest2-kops/gce"
	"k8s.io/kops/tests/e2e/pkg/kops"
	"k8s.io/kops/tests/e2e/pkg/target"
	"k8s.io/kops/tests/e2e/pkg/util"
)

func (d *deployer) init() error {
	var err error
	d.doInit.Do(func() { err = d.initialize(context.TODO()) })
	return err
}

// initialize should only be called by init(), behind a sync.Once
func (d *deployer) initialize(ctx context.Context) error {
	if d.commonOptions.ShouldBuild() {
		if err := d.verifyBuildFlags(); err != nil {
			return fmt.Errorf("init failed to check build flags: %v", err)
		}
	}
	if d.commonOptions.ShouldUp() || d.commonOptions.ShouldDown() {
		if err := d.verifyKopsFlags(); err != nil {
			return fmt.Errorf("init failed to check kops flags: %v", err)
		}
	}
	if d.commonOptions.ShouldUp() {
		if err := d.verifyUpFlags(); err != nil {
			return fmt.Errorf("init failed to check up flags: %v", err)
		}
	}
	if d.KopsVersionMarker != "" {
		d.KopsBinaryPath = path.Join(d.commonOptions.RunDir(), "kops")
		baseURL, err := kops.DownloadKops(d.KopsVersionMarker, d.KopsBinaryPath)
		if err != nil {
			return fmt.Errorf("init failed to download kops from url: %v", err)
		}
		d.KopsBaseURL = baseURL
	}

	switch d.CloudProvider {
	case "aws":
		if d.BoskosResourceType != "" {
			klog.V(1).Info("acquiring AWS credentials from Boskos")

			resource, err := d.boskos.Acquire(ctx, d.BoskosResourceType)
			if err != nil {
				return fmt.Errorf("init failed to get resource %q from boskos: %w", d.BoskosResourceType, err)
			}
			klog.Infof("got AWS account %q from boskos", resource.Name)

			accessKeyIDObj, ok := resource.UserData.Load("access-key-id")
			if !ok {
				return fmt.Errorf("access-key-id not found in boskos resource %q", resource.Name)
			}
			secretAccessKeyObj, ok := resource.UserData.Load("secret-access-key")
			if !ok {
				return fmt.Errorf("secret-access-key not found in boskos resource %q", resource.Name)
			}
			d.awsCredentials = credentials.NewStaticCredentials(accessKeyIDObj.(string), secretAccessKeyObj.(string), "")
			d.createStateStoreBucket = true
		}

		if d.SSHPrivateKeyPath == "" || d.SSHPublicKeyPath == "" {
			publicKeyPath, privateKeyPath, err := util.CreateSSHKeyPair(d.ClusterName)
			if err != nil {
				return err
			}
			d.SSHPublicKeyPath = publicKeyPath
			d.SSHPrivateKeyPath = privateKeyPath
		}
	case "digitalocean":
		if d.SSHPrivateKeyPath == "" {
			d.SSHPrivateKeyPath = os.Getenv("DO_SSH_PRIVATE_KEY_FILE")
		}
		if d.SSHPublicKeyPath == "" {
			d.SSHPublicKeyPath = os.Getenv("DO_SSH_PUBLIC_KEY_FILE")
		}
		d.SSHUser = "root"
	case "gce":
		if d.GCPProject == "" {
			klog.V(1).Info("No GCP project provided, acquiring from Boskos")

			resourceType := "gce-project"
			resource, err := d.boskos.Acquire(ctx, resourceType)
			if err != nil {
				return fmt.Errorf("init failed to get %q resource from boskos: %w", resourceType, err)
			}
			d.GCPProject = resource.Name
			klog.V(1).Infof("Got project %s from boskos", d.GCPProject)

			if d.SSHPrivateKeyPath == "" && d.SSHPublicKeyPath == "" {
				privateKey, publicKey, err := gce.SetupSSH(d.GCPProject)
				if err != nil {
					return err
				}
				d.SSHPrivateKeyPath = privateKey
				d.SSHPublicKeyPath = publicKey
			}
			d.createStateStoreBucket = true
		}
	}

	if err := d.initStateStore(ctx); err != nil {
		return err
	}

	if d.SSHUser == "" {
		d.SSHUser = os.Getenv("KUBE_SSH_USER")
	}
	if d.TerraformVersion != "" {
		t, err := target.NewTerraform(d.TerraformVersion)
		if err != nil {
			return err
		}
		d.terraform = t
	}
	if d.commonOptions.ShouldTest() {
		for _, envvar := range d.env() {
			// Set all of the env vars we use for kops in the current process
			// so that the tester inherits them when shelling out to kops
			if i := strings.Index(envvar, "="); i != -1 {
				os.Setenv(envvar[0:i], envvar[i+1:])
			} else {
				os.Setenv(envvar, "")
			}
		}
	}
	return nil
}

// verifyKopsFlags ensures common fields are set for kops commands
func (d *deployer) verifyKopsFlags() error {
	if d.ClusterName == "" {
		name, err := defaultClusterName(d.CloudProvider)
		if err != nil {
			return err
		}
		d.ClusterName = name
		klog.Infof("Using cluster name: %v", d.ClusterName)
	}

	if d.KopsBinaryPath == "" && d.KopsVersionMarker == "" {
		return errors.New("missing required --kops-binary-path when --kops-version-marker is not used")
	}

	if d.ControlPlaneSize == 0 {
		d.ControlPlaneSize = 1
	}

	switch d.CloudProvider {
	case "aws":
	case "gce":
	case "digitalocean":
	default:
		return errors.New("unsupported --cloud-provider value")
	}

	return nil
}

// env returns a list of environment variables passed to the kops binary
func (d *deployer) env() []string {
	vars := d.Env
	vars = append(vars, []string{
		fmt.Sprintf("PATH=%v", os.Getenv("PATH")),
		fmt.Sprintf("HOME=%v", os.Getenv("HOME")),
		fmt.Sprintf("KOPS_STATE_STORE=%v", d.stateStore),
		fmt.Sprintf("KOPS_FEATURE_FLAGS=%v", d.featureFlags()),
		"KOPS_RUN_TOO_NEW_VERSION=1",
	}...)

	// Pass-through some env vars if set (on all clouds)
	for _, k := range []string{"KOPS_ARCH"} {
		if v := os.Getenv(k); v != "" {
			vars = append(vars, k+"="+v)
		}
	}

	if d.CloudProvider == "aws" {
		// Pass through some env vars if set
		for _, k := range []string{"AWS_PROFILE", "AWS_SHARED_CREDENTIALS_FILE"} {
			v := os.Getenv(k)
			if v != "" {
				vars = append(vars, k+"="+v)
			}
		}
		// Recognized by the e2e framework
		// https://github.com/kubernetes/kubernetes/blob/a750d8054a6cb3167f495829ce3e77ab0ccca48e/test/e2e/framework/ssh/ssh.go#L59-L62
		vars = append(vars, fmt.Sprintf("KUBE_SSH_KEY_PATH=%v", d.SSHPrivateKeyPath))

		if d.awsCredentials != nil {
			credentials, err := d.awsCredentials.Get()
			if err != nil {
				klog.Fatalf("error getting aws credentials: %v", err)
			}
			if credentials.AccessKeyID != "" {
				klog.Infof("setting AWS_ACCESS_KEY_ID")
				vars = append(vars, fmt.Sprintf("AWS_ACCESS_KEY_ID=%v", credentials.AccessKeyID))
			} else {
				klog.Warningf("AWS credentials configured but AWS_ACCESS_KEY_ID was empty")
			}
			if credentials.SecretAccessKey != "" {
				klog.Infof("setting AWS_SECRET_ACCESS_KEY")
				vars = append(vars, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%v", credentials.SecretAccessKey))
			} else {
				klog.Warningf("AWS credentials configured but AWS_SECRET_ACCESS_KEY was empty")
			}
		}
	} else if d.CloudProvider == "digitalocean" {
		// Pass through some env vars if set
		for _, k := range []string{"DIGITALOCEAN_ACCESS_TOKEN", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
			v := os.Getenv(k)
			if v != "" {
				vars = append(vars, k+"="+v)
			} else {
				klog.Warningf("DO env var %s is empty..", k)
			}
		}
	}
	if d.KopsBaseURL != "" {
		vars = append(vars, fmt.Sprintf("KOPS_BASE_URL=%v", d.KopsBaseURL))
	} else if baseURL := os.Getenv("KOPS_BASE_URL"); baseURL != "" {
		vars = append(vars, fmt.Sprintf("KOPS_BASE_URL=%v", os.Getenv("KOPS_BASE_URL")))
	}
	return vars
}

// featureFlags returns the kops feature flags to set
func (d *deployer) featureFlags() string {
	for _, env := range d.Env {
		e := strings.Split(env, "=")
		if e[0] == "KOPS_FEATURE_FLAGS" && len(e) > 1 {
			return e[1]
		}
	}
	return ""
}

// defaultClusterName returns a kops cluster name to use when ClusterName is not set
func defaultClusterName(cloudProvider string) (string, error) {
	jobName := os.Getenv("JOB_NAME")
	jobType := os.Getenv("JOB_TYPE")
	buildID := os.Getenv("BUILD_ID")
	pullNumber := os.Getenv("PULL_NUMBER")
	if jobName == "" || buildID == "" {
		return "", errors.New("JOB_NAME, and BUILD_ID env vars are required when --cluster-name is not set")
	}
	if jobType == "presubmit" && pullNumber == "" {
		return "", errors.New("PULL_NUMBER must be set when JOB_TYPE=presubmit and --cluster-name is not set")
	}

	var suffix string
	switch cloudProvider {
	case "aws":
		suffix = "test-cncf-aws.k8s.io"
	default:
		suffix = "k8s.local"
	}

	if jobType == "presubmit" {
		return fmt.Sprintf("e2e-pr%s.%s.%s", pullNumber, jobName, suffix), nil
	}
	return fmt.Sprintf("e2e-%s.%s", jobName, suffix), nil
}

// initStateStore initializes the kops state store to use
// defaulting to values used in prow jobs,
// but creating a bucket if we are using a dynamic bucket.
func (d *deployer) initStateStore(ctx context.Context) error {
	ss := os.Getenv("KOPS_STATE_STORE")

	switch d.CloudProvider {
	case "aws":
		if d.createStateStoreBucket {
			bucketName, err := aws.AWSBucketName(ctx, d.awsCredentials)
			if err != nil {
				return fmt.Errorf("error building aws bucket name: %w", err)
			}

			if err := aws.EnsureAWSBucket(ctx, d.awsCredentials, bucketName); err != nil {
				return err
			}

			ss = "s3://" + bucketName
		} else {
			if ss == "" {
				ss = "s3://k8s-kops-prow"
			}
		}
	case "gce":
		if d.createStateStoreBucket {
			ss = "gs://" + gce.GCSBucketName(d.GCPProject)
			if err := gce.EnsureGCSBucket(ss, d.GCPProject); err != nil {
				return err
			}
		}
	case "digitalocean":
		ss = "do://e2e-kops-space"

	default:
		if d.createStateStoreBucket {
			return fmt.Errorf("bucket creation not implemented for cloud %q", d.CloudProvider)
		}
	}

	if ss == "" {
		return fmt.Errorf("cannot determine KOPS_STATE_STORE")
	}

	d.stateStore = ss
	return nil
}

// the default is $ARTIFACTS if set, otherwise ./_artifacts
// constructed as an absolute path to help the ginkgo tester because
// for some reason it needs an absolute path to the kubeconfig
func defaultArtifactsDir() (string, error) {
	if path, set := os.LookupEnv("ARTIFACTS"); set {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to convert filepath from $ARTIFACTS (%s) to absolute path: %s", path, err)
		}
		return absPath, nil
	}

	absPath, err := filepath.Abs("_artifacts")
	if err != nil {
		return "", fmt.Errorf("when constructing default artifacts dir, failed to get absolute path: %s", err)
	}
	return absPath, nil
}
