/*
Copyright 2019 The Kubernetes Authors.

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

package vfs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"k8s.io/klog/v2"
)

// matches regional naming conventions of S3:
// https://docs.aws.amazon.com/general/latest/gr/s3.html
// TODO: match fips and S3 access point naming conventions
// TODO: perhaps make region regex more specific, i.e. (us|eu|ap|cn|ca|sa), to prevent matching bucket names that match region format?
//
//	but that will mean updating this list when AWS introduces new regions
var s3UrlRegexp = regexp.MustCompile(`(s3([-.](?P<region>\w{2}(-gov)?-\w+-\d{1})|[-.](?P<bucket>[\w.\-\_]+)|)?|(?P<bucket>[\w.\-\_]+)[.]s3([.-](?P<region>\w{2}(-gov)?-\w+-\d{1}))?)[.]amazonaws[.]com([.]cn)?(?P<path>.*)?`)

type S3BucketDetails struct {
	// context is the S3Context we are associated with
	context *S3Context

	// region is the region we have determined for the bucket
	region string

	// name is the name of the bucket
	name string

	// mutex protects applyServerSideEncryptionByDefault
	mutex sync.Mutex

	// applyServerSideEncryptionByDefault caches information on whether server-side encryption is enabled on the bucket
	applyServerSideEncryptionByDefault *bool
}

type S3Context struct {
	mutex         sync.Mutex
	clients       map[string]*s3.Client
	bucketDetails map[string]*S3BucketDetails
}

func NewS3Context() *S3Context {
	return &S3Context{
		clients:       make(map[string]*s3.Client),
		bucketDetails: make(map[string]*S3BucketDetails),
	}
}

type ResolverV2 struct{}

func (*ResolverV2) ResolveEndpoint(ctx context.Context, params s3.EndpointParameters) (
	smithyendpoints.Endpoint, error,
) {
	params.UseDualStack = aws.Bool(true)
	return s3.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
}

func (s *S3Context) getClient(ctx context.Context, region string) (*s3.Client, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s3Client := s.clients[region]
	if s3Client == nil {
		_, span := tracer.Start(ctx, "S3Context::getClient")
		defer span.End()

		var config aws.Config
		var err error
		endpoint := os.Getenv("S3_ENDPOINT")
		if endpoint == "" {
			config, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
			if err != nil {
				return nil, fmt.Errorf("error loading AWS config: %v", err)
			}
		} else {
			// Use customized S3 storage
			klog.V(2).Infof("Found S3_ENDPOINT=%q, using as non-AWS S3 backend", endpoint)
			config, err = getCustomS3Config(ctx, region)
			if err != nil {
				return nil, err
			}
		}

		s3Client = s3.NewFromConfig(config, func(o *s3.Options) {
			if endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
				o.UsePathStyle = true
			} else {
				o.EndpointResolverV2 = &ResolverV2{}
			}
		})
		s.clients[region] = s3Client
	}

	return s3Client, nil
}

func getCustomS3Config(ctx context.Context, region string) (aws.Config, error) {
	accessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	if accessKeyID == "" {
		return aws.Config{}, fmt.Errorf("S3_ACCESS_KEY_ID cannot be empty when S3_ENDPOINT is not empty")
	}
	secretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")
	if secretAccessKey == "" {
		return aws.Config{}, fmt.Errorf("S3_SECRET_ACCESS_KEY cannot be empty when S3_ENDPOINT is not empty")
	}

	s3Config, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("error loading AWS config: %v", err)
	}
	return s3Config, nil
}

func (s *S3Context) getDetailsForBucket(ctx context.Context, bucket string) (*S3BucketDetails, error) {
	s.mutex.Lock()
	bucketDetails := s.bucketDetails[bucket]
	s.mutex.Unlock()

	if bucketDetails != nil && bucketDetails.region != "" {
		return bucketDetails, nil
	}

	ctx, span := tracer.Start(ctx, "S3Path::getDetailsForBucket")
	defer span.End()

	bucketDetails = &S3BucketDetails{
		context: s,
		name:    bucket,
	}

	// Probe to find correct region for bucket
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint != "" {

		// If customized S3 storage is set, return user-defined region
		bucketDetails.region = os.Getenv("S3_REGION")
		if bucketDetails.region == "" {
			bucketDetails.region = "us-east-1"
		}
		return bucketDetails, nil
	}

	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		isEC2, err := isRunningOnEC2(ctx)
		if isEC2 || err != nil {
			region, err := getRegionFromMetadata(ctx)
			if err != nil {
				klog.V(2).Infof("unable to get region from metadata:%v", err)
			} else {
				awsRegion = region
				klog.V(2).Infof("got region from metadata: %q", awsRegion)
			}
		}
	}

	if awsRegion == "" {
		awsRegion = "us-east-1"
		klog.V(2).Infof("defaulting region to %q", awsRegion)
	}

	bucketLocation := ""

	// First, try a HEAD
	if bucketLocation == "" {
		region, err := getBucketRegionFromHeadRequest(ctx, bucket)
		if err != nil {
			klog.V(2).Infof("unable to get location for bucket %q from HEAD request: %v", bucket, err)
		}
		bucketLocation = region
	}

	if bucketLocation == "" {
		request := &s3.GetBucketLocationInput{
			Bucket: &bucket,
		}
		var response *s3.GetBucketLocationOutput

		s3Client, err := s.getClient(ctx, awsRegion)
		if err != nil {
			return bucketDetails, fmt.Errorf("error connecting to S3: %s", err)
		}
		// Attempt one GetBucketLocation call the "normal" way (i.e. as the bucket owner)
		response, err = s3Client.GetBucketLocation(ctx, request)

		// and fallback to brute-forcing if it fails
		if err != nil {
			klog.V(2).Infof("unable to get location for bucket %q from region %q; scanning all regions: %v", bucket, awsRegion, err)
		}

		if len(response.LocationConstraint) == 0 {
			// US Classic does not return a region
			bucketLocation = "us-east-1"
		} else {
			bucketLocation = string(response.LocationConstraint)
		}
	}

	if bucketLocation == "" {
		request := &s3.GetBucketLocationInput{
			Bucket: &bucket,
		}

		response, err := bruteforceBucketLocation(ctx, awsRegion, request)
		if err != nil {
			return bucketDetails, err
		}
		if len(response.LocationConstraint) == 0 {
			// US Classic does not return a region
			bucketLocation = "us-east-1"
		} else {
			bucketLocation = string(response.LocationConstraint)
		}
	}

	if bucketLocation == "" {
		return nil, fmt.Errorf("unable to determine location for bucket %q", bucket)
	}

	// Another special case: "EU" can mean eu-west-1
	if bucketLocation == "EU" {
		bucketDetails.region = "eu-west-1"
	} else {
		bucketDetails.region = bucketLocation
	}

	klog.V(2).Infof("found bucket in region %q", bucketDetails.region)

	s.mutex.Lock()
	s.bucketDetails[bucket] = bucketDetails
	s.mutex.Unlock()

	return bucketDetails, nil
}

func getBucketRegionFromHeadRequest(ctx context.Context, bucket string) (string, error) {
	url := fmt.Sprintf("https://%s.s3.amazonaws.com", bucket)
	response, err := http.Head(url)
	if err != nil {
		return "", fmt.Errorf("doing HEAD request against %q: %w", url, err)
	}
	region := response.Header.Get("X-Amz-Bucket-Region")
	if region == "" {
		return "", fmt.Errorf("header X-Amz-Bucket-Region not returned in url %q", url)
	}
	return region, nil
}

func (b *S3BucketDetails) hasServerSideEncryptionByDefault(ctx context.Context) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.applyServerSideEncryptionByDefault != nil {
		return *b.applyServerSideEncryptionByDefault
	}

	ctx, span := tracer.Start(ctx, "S3BucketDetails::hasServerSideEncryptionByDefault")
	defer span.End()

	applyServerSideEncryptionByDefault := false

	// We only make one attempt to find the SSE policy (even if there's an error)
	b.applyServerSideEncryptionByDefault = &applyServerSideEncryptionByDefault

	client, err := b.context.getClient(ctx, b.region)
	if err != nil {
		klog.Warningf("Unable to read bucket encryption policy for %q in region %q: will encrypt using AES256", b.name, b.region)
		return false
	}

	klog.V(4).Infof("Checking default bucket encryption for %q", b.name)

	request := &s3.GetBucketEncryptionInput{}
	request.Bucket = aws.String(b.name)

	klog.V(8).Infof("Calling S3 GetBucketEncryption Bucket=%q", b.name)

	result, err := client.GetBucketEncryption(ctx, request)
	if err != nil {
		// the following cases might lead to the operation failing:
		// 1. A deny policy on s3:GetEncryptionConfiguration
		// 2. No default encryption policy set
		klog.V(8).Infof("Unable to read bucket encryption policy for %q: will encrypt using AES256", b.name)
		return false
	}

	// currently, only one element is in the rules array, iterating nonetheless for future compatibility
	for _, element := range result.ServerSideEncryptionConfiguration.Rules {
		if element.ApplyServerSideEncryptionByDefault != nil {
			applyServerSideEncryptionByDefault = true
		}
	}

	b.applyServerSideEncryptionByDefault = &applyServerSideEncryptionByDefault

	klog.V(2).Infof("bucket %q has default encryption set to %t", b.name, applyServerSideEncryptionByDefault)

	return applyServerSideEncryptionByDefault
}

/*
Amazon's S3 API provides the GetBucketLocation call to determine the region in which a bucket is located.
This call can however only be used globally by the owner of the bucket, as mentioned on the documentation page.

For S3 buckets that are shared across multiple AWS accounts using bucket policies the call will only work if it is sent
to the correct region in the first place.

This method will attempt to "bruteforce" the bucket location by sending a request to every available region and picking
out the first result.

See also: https://docs.aws.amazon.com/goto/WebAPI/s3-2006-03-01/GetBucketLocationRequest
*/
func bruteforceBucketLocation(ctx context.Context, region string, request *s3.GetBucketLocationInput) (*s3.GetBucketLocationOutput, error) {
	ctx, span := tracer.Start(ctx, "bruteforceBucketLocation")
	defer span.End()

	config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("creating aws config: %w", err)
	}
	//config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(*config.Region), awsconfig.WithSharedCredentialsFiles())

	regions, err := ec2.NewFromConfig(config).DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("listing AWS regions: %w", err)
	}

	klog.V(2).Infof("Querying S3 for bucket location for %s", *request.Bucket)

	out := make(chan *s3.GetBucketLocationOutput, len(regions.Regions))
	for _, region := range regions.Regions {
		go func(regionName string) {
			config, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(regionName))
			if err == nil {
				klog.V(8).Infof("Doing GetBucketLocation in %q", regionName)
				s3Client := s3.NewFromConfig(config)
				result, bucketError := s3Client.GetBucketLocation(ctx, request)
				if bucketError == nil {
					klog.V(8).Infof("GetBucketLocation succeeded in %q", regionName)
					out <- result
				}
			}
		}(*region.RegionName)
	}

	select {
	case bucketLocation := <-out:
		return bucketLocation, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("could not retrieve location for AWS bucket %q", *request.Bucket)
	}
}

// isRunningOnEC2 determines if we could be running on EC2.
// It is used to avoid a call to the metadata service to get the current region,
// because that call is slow if not running on EC2
func isRunningOnEC2(ctx context.Context) (bool, error) {
	if runtime.GOOS == "linux" {
		// Approach based on https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/identify_ec2_instances.html
		productUUID, err := os.ReadFile("/sys/devices/virtual/dmi/id/product_uuid")
		if err != nil {
			klog.V(2).Infof("unable to read /sys/devices/virtual/dmi/id/product_uuid, assuming not running on EC2: %v", err)
			return false, nil
		}

		s := strings.ToLower(strings.TrimSpace(string(productUUID)))
		if strings.HasPrefix(s, "ec2") {
			klog.V(2).Infof("product_uuid is %q, assuming running on EC2", s)
			return true, nil
		}
		klog.V(2).Infof("product_uuid is %q, assuming not running on EC2", s)
		return false, nil
	}
	klog.V(2).Infof("GOOS=%q, assuming not running on EC2", runtime.GOOS)
	return false, nil
}

// getRegionFromMetadata queries the metadata service for the current region, if running in EC2
func getRegionFromMetadata(ctx context.Context) (string, error) {
	ctx, span := tracer.Start(ctx, "getRegionFromMetadata")
	defer span.End()

	// Use an even shorter timeout, to minimize impact when not running on EC2
	// Note that we still retry a few times, this works out a little under a 1s delay
	shortTimeout := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithHTTPClient(shortTimeout))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := imds.NewFromConfig(config)

	metadataRegion, err := client.GetRegion(ctx, &imds.GetRegionInput{})
	if err != nil {
		return "", fmt.Errorf("getting AWS region from metadata: %w", err)
	}

	return metadataRegion.Region, nil
}

func VFSPath(url string) (string, error) {
	if !s3UrlRegexp.MatchString(url) {
		return "", fmt.Errorf("%s is not a valid S3 URL", url)
	}
	groupNames := s3UrlRegexp.SubexpNames()
	result := s3UrlRegexp.FindAllStringSubmatch(url, -1)[0]

	captured := map[string]string{}
	for i, value := range result {
		if value != "" {
			captured[groupNames[i]] = value
		}
	}
	bucket := captured["bucket"]
	path := captured["path"]
	if bucket == "" {
		if path == "" {
			return "", fmt.Errorf("invalid S3 url %q (no bucket defined)", url)
		}
		return fmt.Sprintf("s3:/%s", path), nil
	}
	return fmt.Sprintf("s3://%s%s", bucket, path), nil
}
