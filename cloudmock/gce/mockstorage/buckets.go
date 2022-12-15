/*
Copyright 2021 The Kubernetes Authors.

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

package mockstorage

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/api/storage/v1"
	"k8s.io/kops/cloudmock/gce/gcphttp"
)

type buckets struct {
	mutex sync.Mutex

	buckets  map[string]*bucket
	policies map[string]*storage.Policy
}

type bucket struct {
	data    *storage.Bucket
	objects map[string]*object
}

type object struct {
	meta storage.Object
	data []byte
}

func (s *buckets) Init() {
	s.buckets = make(map[string]*bucket)
	s.policies = make(map[string]*storage.Policy)
}

func (s *buckets) createBucket(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return gcphttp.ErrorBadRequest("")
	}

	bucketObj := &storage.Bucket{}
	if err := json.Unmarshal(body, &bucketObj); err != nil {
		return gcphttp.ErrorBadRequest("")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	b := s.buckets[bucketObj.Name]
	if b != nil {
		return gcphttp.ErrorAlreadyExists("")
	}

	b = &bucket{
		data:    bucketObj,
		objects: make(map[string]*object),
	}
	s.buckets[bucketObj.Name] = b

	return gcphttp.OKResponse(bucketObj)
}

func (s *buckets) getBucket(bucketName string, request *http.Request) (*http.Response, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return gcphttp.ErrorNotFound("")
	}

	return gcphttp.OKResponse(bucket.data)
}
    integration_test.go:1063: error running update cluster "minimal-gce.example.com": error retrieving SSH public key "admin": error listing gs://mock-state-bucket/minimal-gce.example.com/pki/ssh/public/admin: Get "https://storage.googleapis.com/storage/v1/b/mock-state-bucket/o?alt=json&delimiter=%2F&prefix=minimal-gce.example.com%2Fpki%2Fssh%2Fpublic%2Fadmin%2F&prettyPrint=false": unhandled request &http.Request{Method:"GET", URL:(*url.URL)(0xc000e89320), Proto:"HTTP/1.1", ProtoMajor:1, ProtoMinor:1, Header:http.Header{"User-Agent":[]string{"google-api-go-client/0.5"}, "X-Goog-Api-Client":[]string{"gl-go/1.19.1 gdcl/0.104.0"}}, Body:io.ReadCloser(nil), GetBody:(func() (io.ReadCloser, error))(nil), ContentLength:0, TransferEncoding:[]string(nil), Close:false, Host:"storage.googleapis.com", Form:url.Values(nil), PostForm:url.Values(nil), MultipartForm:(*multipart.Form)(nil), Trailer:http.Header(nil), RemoteAddr:"", RequestURI:"", TLS:(*tls.ConnectionState)(nil), Cancel:(<-chan struct {})(nil), Response:(*http.Response)(nil), ctx:(*context.emptyCtx)(0xc000056098)}

func (s *buckets) createObject(ctx context.Context, bucketName string, req *http.Request) (*http.Response, error) {
	query := req.URL.Query()

	uploadType := query.Get("uploadType")
	if uploadType == "" {
		return gcphttp.ErrorBadRequest("uploadType is required")
	}

	contentType := req.Header.Get("content-type")
	if contentType == "" {
		return gcphttp.ErrorBadRequest("contentType is required")
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return gcphttp.ErrorBadRequest("invalid content-type header")
	}

	obj := &object{}

	if uploadType == "multipart" {
		if !strings.HasPrefix(mediaType, "multipart/") {
			return gcphttp.ErrorBadRequest("invalid content-type header (expected multipart)")
		}
		boundary := params["boundary"]
		if boundary == "" {
			return gcphttp.ErrorBadRequest("boundary is required")
		}
		mr := multipart.NewReader(req.Body, boundary)
		i := 0
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return gcphttp.ErrorBadRequest("invalid multipart request")
			}

			body, err := io.ReadAll(p)
			if err != nil {
				return gcphttp.ErrorBadRequest("invalid multipart request")
			}

			switch i {
			case 0:
				if err := json.Unmarshal(body, &obj.meta); err != nil {
					return gcphttp.ErrorBadRequest("invalid multipart request (bad metadata)")
				}

			case 1:
				obj.data = body
			default:
				return gcphttp.ErrorBadRequest("invalid multipart request (too many parts)")
			}
			i++
		}
	} else {
		return gcphttp.ErrorBadRequest("invalid uploadType")
	}

	if obj.meta.Name == "" {
		return gcphttp.ErrorBadRequest("name is required")
	}
	obj.meta.Kind = "storage#object"

	s.mutex.Lock()
	defer s.mutex.Unlock()

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return gcphttp.ErrorNotFound("")
	}

	bucket.objects[obj.meta.Name] = obj

	return gcphttp.OKResponse(&obj.meta)
}

func (s *buckets) getIAMPolicy(bucketName string, request *http.Request) (*http.Response, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bucket := s.buckets[bucketName]
	if bucket == nil {
		return gcphttp.ErrorNotFound("")
	}

	policy := s.policies[bucketName]
	if policy == nil {
		policy = &storage.Policy{}
	}

	return gcphttp.OKResponse(policy)
}

func (s *buckets) setIAMPolicy(bucket string, request *http.Request) (*http.Response, error) {
	b, err := io.ReadAll(request.Body)
	if err != nil {
		return gcphttp.ErrorBadRequest("")
	}

	req := &storage.Policy{}
	if err := json.Unmarshal(b, &req); err != nil {
		return gcphttp.ErrorBadRequest("")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// TODO: etag

	policy := req
	s.policies[bucket] = policy

	return gcphttp.OKResponse(policy)
}
