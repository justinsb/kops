/*
Copyright 2022 The Kubernetes Authors.

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

package mocks3

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/private/protocol/xml/xmlutil"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"k8s.io/klog/v2"
)

// mockS3 represents a mocked S3 service
type mockS3 struct {
	buckets map[string]*s3Bucket
}

type s3Bucket struct {
	objects map[string]*s3Object
}

type s3Object struct {
	contents []byte
}

// New creates a new mock IAM client.
func New() *mockS3 {
	s := &mockS3{
		buckets: make(map[string]*s3Bucket),
	}
	return s
}

func (s *mockS3) HTTPClient() *http.Client {
	return &http.Client{
		Transport: s,
	}
}

func (s *mockS3) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := request.ParseForm(); err != nil {
		klog.Warningf("failed to parse form: %v", err)
	}

	// var body []byte
	// if request.Body != nil {
	// 	b, err := io.ReadAll(request.Body)
	// 	if err != nil {
	// 		klog.Warningf("failed to read body: %v", err)
	// 	}
	// 	body = b
	// }
	klog.Infof("request %s %s: %s", request.Method, request.URL, request.Form)

	url := request.URL
	if url.Host == "ec2.us-east-1.amazonaws.com" {
		action := request.Form.Get("Action")
		if action == "DescribeRegions" {

			response := ec2.DescribeRegionsOutput{}
			response.Regions = append(response.Regions, &ec2.Region{
				RegionName: aws.String("us-east-1"),
			})

			// 			<?xml version="1.0" encoding="UTF-8"?>\n<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/
			// doc/2016-11-15/">\n    <requestId>b91bb109-e13a-4a95-8822-69baa5d901ca</requestId>\n    <regionInfo
			// >\n        <item>\n            <regionName>eu-north-1</regionName>\n

			requestID := "b91bb109-e13a-4a95-8822-69baa5d901ca"
			var buf bytes.Buffer

			buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
			buf.WriteString("<DescribeRegionsResponse xmlns=\"http://ec2.amazonaws.com/doc/2016-11-15/\">")
			buf.WriteString("<requestId>" + requestID + "</requestId>")
			err := xmlutil.BuildXML(response, xml.NewEncoder(&buf))
			if err != nil {
				klog.Warningf("failed to encode response: %v", err)
			}
			buf.WriteString("</DescribeRegionsResponse>")

			// klog.Infof("response is %s", buf.String())

			body := ioutil.NopCloser(&buf)
			httpResponse := &http.Response{
				Status:     http.StatusText(http.StatusOK),
				StatusCode: http.StatusOK,
				Body:       body,
				Header:     make(http.Header),
			}
			httpResponse.Header.Add("Content-Type", "text/xml;charset=UTF-8")
			httpResponse.Header.Add("x-amzn-RequestId", requestID)
			return httpResponse, nil
		}
	}

	if url.Host == "s3.amazonaws.com" || url.Host == "s3.dualstack.us-east-1.amazonaws.com" {
		if request.Method == "GET" {
			if _, ok := request.Form["location"]; ok {
				bucket := strings.TrimPrefix(request.URL.Path, "/")
				return s.getBucketLocation(request, bucket)
			}
		}
	}

	if url.Host == "example-bucket.s3.dualstack.us-east-1.amazonaws.com" || url.Host == "example-bucket.s3.amazonaws.com" {
		objectKey := strings.TrimPrefix(request.URL.Path, "/")
		bucket := "example-bucket"

		if request.Method == "PUT" {
			if objectKey == "" {
				return s.createBucket(request, bucket)
			}
			return s.putObject(request, bucket, objectKey)
		}

		if request.Method == "GET" {
			return s.getObject(request, bucket, objectKey)
		}
	}

	klog.Warningf("404 request: %s %s %#v", request.Method, request.URL, request)
	return nil, fmt.Errorf("unhandled request %#v", request)
}

func (s *mockS3) bucketNotFound() (*http.Response, error) {
	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusNotFound),
		StatusCode: http.StatusNotFound,
		Header:     make(http.Header),
	}
	// httpResponse.Header.Add("Content-Type", "application/xml")
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil
}

func (s *mockS3) bucketAlreadyExists() (*http.Response, error) {
	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusConflict),
		StatusCode: http.StatusConflict,
		Header:     make(http.Header),
	}
	// httpResponse.Header.Add("Content-Type", "application/xml")
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil
}

func (s *mockS3) putObject(request *http.Request, bucketName, objectKey string) (*http.Response, error) {
	klog.Infof("PutObject %s %s", bucketName, objectKey)

	body, err := ioutil.ReadAll(request.Body)
	if err != nil {
		klog.Warningf("failed to read body: %v", err)
	}

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return s.bucketNotFound()
	}
	bucket.objects[objectKey] = &s3Object{
		contents: body,
	}

	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		// Body:       body,
		Header: make(http.Header),
	}
	// httpResponse.Header.Add("Content-Type", "application/xml")
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil

}

func (s *mockS3) createBucket(request *http.Request, bucketName string) (*http.Response, error) {
	klog.Infof("CreateBucket %s", bucketName)

	bucket := s.buckets[bucketName]
	if bucket != nil {
		return s.bucketAlreadyExists()
	}

	s.buckets[bucketName] = &s3Bucket{
		objects: make(map[string]*s3Object),
	}

	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		// Body:       body,
		Header: make(http.Header),
	}
	httpResponse.Header.Add("Location", "/"+bucketName)
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil

}

func (s *mockS3) getObject(request *http.Request, bucketName, objectKey string) (*http.Response, error) {

	klog.Infof("GetObject %s %s", bucketName, objectKey)

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return s.bucketNotFound()
	}

	obj := bucket.objects[objectKey]
	if obj == nil {
		httpResponse := &http.Response{
			Status:     http.StatusText(http.StatusNotFound),
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
		}
		// httpResponse.Header.Add("Content-Type", "application/xml")
		// httpResponse.Header.Add("x-amzn-request-id", requestID)
		return httpResponse, nil
	}

	body := ioutil.NopCloser(bytes.NewReader(obj.contents))

	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     make(http.Header),
	}
	// httpResponse.Header.Add("Content-Type", "application/xml")
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil

}

func (s *mockS3) getBucketLocation(r *http.Request, bucketName string) (*http.Response, error) {
	klog.Infof("GetBucketLocation %q", bucketName)

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return s.bucketNotFound()
	}

	// GetBucketLocation
	response := s3.GetBucketLocationOutput{}

	// No constraint => us-east-1

	var buf bytes.Buffer

	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")

	err := xmlutil.BuildXML(response, xml.NewEncoder(&buf))
	if err != nil {
		klog.Warningf("failed to encode response: %v", err)
	}

	klog.Infof("response is %s", buf.String())

	body := ioutil.NopCloser(&buf)
	httpResponse := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     make(http.Header),
	}
	httpResponse.Header.Add("Content-Type", "application/xml")
	// httpResponse.Header.Add("x-amzn-request-id", requestID)
	return httpResponse, nil
}
