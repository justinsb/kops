package kustomize

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/kops/pkg/diff"
)

func TestBuildFromUrl(t *testing.T) {
	base := "https://raw.githubusercontent.com/kubernetes/kubectl/master/cmd/kustomize/demos/data/helloWorld/"

	l := NewHttpLoader()

	b, err := Build(base, l)
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	var expected = `
apiVersion: v1
data:
  altGreeting: Good Morning!
  enableRisky: "false"
kind: ConfigMap
metadata:
  labels:
    app: hello
  name: the-map-mdcf9kh4c2
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: hello
  name: the-service
spec:
  ports:
  - port: 8666
    protocol: TCP
    targetPort: 8080
  selector:
    app: hello
    deployment: hello
  type: LoadBalancer
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: hello
  name: the-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hello
  template:
    metadata:
      labels:
        app: hello
        deployment: hello
    spec:
      containers:
      - command:
        - /hello
        - --port=8080
        - --enableRiskyFeature=$(ENABLE_RISKY)
        env:
        - name: ALT_GREETING
          valueFrom:
            configMapKeyRef:
              key: altGreeting
              name: the-map-mdcf9kh4c2
        - name: ENABLE_RISKY
          valueFrom:
            configMapKeyRef:
              key: enableRisky
              name: the-map-mdcf9kh4c2
        image: monopole/hello:1
        name: the-container
        ports:
        - containerPort: 8080
`

	expected = strings.TrimSpace(expected)
	actual := strings.TrimSpace(string(b))
	if actual != expected {
		t.Errorf("unexpected result from Build: %s", diff.FormatDiff(expected, actual))
	}

	files, err := Walk(base, l)
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)

	expectedPaths := []string{base + "configMap.yaml", base + "deployment.yaml", base + "kustomization.yaml", base + "service.yaml"}
	if !reflect.DeepEqual(paths, expectedPaths) {
		t.Errorf("unexpected paths from Walk: %s vs %s", paths, expectedPaths)
	}
}

func TestTraversal(t *testing.T) {
	base := "https://raw.githubusercontent.com/kubernetes/kubectl/master/cmd/kustomize/demos/data/ldap/"
	l := NewHttpLoader()

	files, err := Walk(base+"overlays/production/", l)
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	sort.Strings(paths)

	expectedPaths := []string{base + "base/deployment.yaml", base + "base/env.startup.txt", base + "base/kustomization.yaml", base + "base/service.yaml",
		base + "overlays/production/deployment.yaml",
		base + "overlays/production/kustomization.yaml",
	}
	if !reflect.DeepEqual(paths, expectedPaths) {
		t.Errorf("unexpected paths from Walk: %s vs %s", paths, expectedPaths)
	}
}
