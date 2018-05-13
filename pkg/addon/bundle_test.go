package addon

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/kops/pkg/addon/kustomize"
	"k8s.io/kops/pkg/diff"
)

func TestCreateConfigMap(t *testing.T) {
	base := "https://raw.githubusercontent.com/kubernetes/kubectl/master/cmd/kustomize/demos/data/helloWorld/"

	l := kustomize.NewHttpLoader()

	configmap, err := createConfigMap("fakechannel", base, l)
	if err != nil {
		t.Fatalf("unexpected error from CreateConfigMap: %v", err)
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

	a := &Bundle{
		configMap: configmap,
	}

	b, err := a.Manifest()
	if err != nil {
		t.Fatalf("error from Build: %v", err)
	}

	expected = strings.TrimSpace(expected)
	actual := strings.TrimSpace(string(b))
	if actual != expected {
		t.Errorf("unexpected result from Build: %s", diff.FormatDiff(expected, actual))
	}

	var keys []string
	for k := range configmap.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	expectedKeys := []string{"_system/base/configMap.yaml", "_system/base/deployment.yaml", "_system/base/kustomization.yaml", "_system/base/service.yaml", "_system/channel", "_system/current"}
	if !reflect.DeepEqual(keys, expectedKeys) {
		t.Errorf("unexpected keys in configmap: %s vs %s", keys, expectedKeys)
	}
}
