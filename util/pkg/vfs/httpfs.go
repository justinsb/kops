/*
Copyright 2018 The Kubernetes Authors.

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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"
)

type HTTPPath struct {
	u url.URL
}

var _ Path = &S3Path{}

func newHTTPPath(u *url.URL) *HTTPPath {
	return &HTTPPath{
		u: *u,
	}
}

func (p *HTTPPath) Path() string {
	return p.u.String()
}

func (p *HTTPPath) String() string {
	return p.Path()
}

func (p *HTTPPath) Remove() error {
	return fmt.Errorf("cannot remove an HTTP vfs file %s", p)
}

func (p *HTTPPath) Join(relativePath ...string) Path {
	u := p.u
	for _, rp := range relativePath {
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		u.Path += rp
	}
	return &HTTPPath{
		u: u,
	}
}

func (p *HTTPPath) WriteFile(data io.ReadSeeker, aclObj ACL) error {
	return fmt.Errorf("cannot write to an HTTP vfs file %s", p)
}

func (p *HTTPPath) CreateFile(data io.ReadSeeker, acl ACL) error {
	return fmt.Errorf("cannot write to an HTTP vfs file %s", p)
}

// ReadFile implements Path::ReadFile
func (p *HTTPPath) ReadFile() ([]byte, error) {
	var b bytes.Buffer
	_, err := p.WriteTo(&b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteTo implements io.WriterTo
func (p *HTTPPath) WriteTo(out io.Writer) (int64, error) {
	httpURL := p.u.String()

	glog.V(4).Infof("Performing HTTP request: GET %s", httpURL)
	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return 0, err
	}
	response, err := http.DefaultClient.Do(req)
	if response != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return 0, fmt.Errorf("error fetching %q: %v", httpURL, err)
	}
	if response.StatusCode == 404 {
		return 0, os.ErrNotExist
	}

	if response.StatusCode != 200 {
		return 0, fmt.Errorf("unexpected response code from %s: %s", httpURL, response.Status)
	}

	n, err := io.Copy(out, response.Body)
	if err != nil {
		return n, fmt.Errorf("error reading %s: %v", p, err)
	}
	return n, nil
}

func (p *HTTPPath) ReadDir() ([]Path, error) {
	return nil, fmt.Errorf("cannot list directories over http (%s)", p)
}

func (p *HTTPPath) ReadTree() ([]Path, error) {
	return nil, fmt.Errorf("cannot list directories over http (%s)", p)
}

func (p *HTTPPath) Base() string {
	return path.Base(p.u.Path)
}
