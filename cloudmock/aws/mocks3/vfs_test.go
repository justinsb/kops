package mocks3

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"k8s.io/klog/v2"
	"k8s.io/kops/util/pkg/vfs"
)

func TestVFS(t *testing.T) {
	// klog.InitFlags(nil)
	// flag.Set("v", "8")
	// flag.Parse()

	// By setting up these env vars, we avoid any lookups to metadata etc
	os.Setenv("AWS_ACCESS_KEY_ID", "fake-access-key-id")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "fake-access-key-secret")
	os.Setenv("AWS_REGION", "us-east-1")

	mockS3 := New()
	httpClient := mockS3.HTTPClient()
	vfs.Context = vfs.NewVFSContext(httpClient)

	if err := createBucket(httpClient, "example-bucket"); err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	p, err := vfs.Context.BuildVfsPath("s3://example-bucket/myfile")
	if err != nil {
		t.Fatalf("BuildVfsPath failed: %v", err)
	}

	want := []byte("hello-world")
	if err := p.WriteFile(bytes.NewReader(want), nil); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := p.ReadFile()
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected results from reading file; got %s, want %s", string(got), string(want))
	}
}

func createBucket(httpClient *http.Client, bucketName string) error {
	config := aws.NewConfig()
	config = config.WithCredentialsChainVerboseErrors(true)

	config.Region = aws.String("us-east-1")
	config.HTTPClient = httpClient

	session, err := session.NewSessionWithOptions(session.Options{
		Config:            *config,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return fmt.Errorf("error starting new AWS session: %v", err)
	}

	s3Client := s3.New(session, config)
	output, err := s3Client.CreateBucket(&s3.CreateBucketInput{Bucket: &bucketName})
	klog.Infof("create bucket returned %#v", output)
	return err
}
