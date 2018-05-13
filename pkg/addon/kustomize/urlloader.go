package kustomize

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	"k8s.io/kubectl/pkg/loader"
)

// // Interface for different types of loaders (e.g. fileLoader, httpLoader, etc.)
// type SchemeLoader interface {
// 	// Does this location correspond to this scheme.
// 	IsScheme(root string, location string) bool
// 	// Combines the root and path into a full location string.
// 	FullLocation(root string, path string) (string, error)
// 	// Load bytes at scheme-specific location or an error.
// 	Load(location string) ([]byte, error)
// }

type httpLoader struct {
}

var _ loader.SchemeLoader = &httpLoader{}

// NewHttpLoader returns a SchemeLoader to handle http / https URLs
func NewHttpLoader() loader.SchemeLoader {
	return &httpLoader{}
}

// Is the location calculated with the root and location params a full file path.
func (l *httpLoader) IsScheme(root string, location string) bool {
	fullPath, err := l.FullLocation(root, location)
	if err != nil {
		return false
	}
	if fullPath == "" {
		return false
	}
	return true
}

func (l *httpLoader) FullLocation(root string, location string) (string, error) {
	// First, validate the parameters
	if len(root) == 0 && len(location) == 0 {
		return "", fmt.Errorf("Unable to calculate full location: root and location empty")
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("error parsing location %q", location)
	}

	if u.IsAbs() {
		return u.String(), err
	}

	if !strings.HasSuffix(root, "/") {
		root += "/"
	}

	rootURL, err := url.Parse(root)
	if err != nil {
		return "", fmt.Errorf("error parsing root %q", root)
	}

	r := rootURL.ResolveReference(u)
	glog.V(4).Infof("FullLocation(%s, %s) => %s", root, location, r)

	return r.String(), nil
}

// Load returns the bytes from reading a file at fullFilePath.
// Implements the Loader interface.
func (l *httpLoader) Load(fullPath string) ([]byte, error) {
	u, err := url.Parse(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error parsing full path %q as URL", fullPath)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unexpected scheme in path %q", fullPath)
	}

	// TODO: Mirror retry / backoff logic from vfs
	glog.V(4).Infof("Performing HTTP request: GET %s", fullPath)
	resp, err := http.Get(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error fetching %q: %v", fullPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response %q from %q", resp.Status, fullPath)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from %q: %v", fullPath, err)
	}

	return body, nil
}
