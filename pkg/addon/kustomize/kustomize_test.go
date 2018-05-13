package kustomize

import (
	"strings"
	"testing"

	"k8s.io/kops/pkg/diff"
	"k8s.io/kubectl/pkg/kustomize/util/fs"
	"k8s.io/kubectl/pkg/loader"
)

var service = []byte(`
apiVersion: v1
kind: Service
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: coredns
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
    kubernetes.io/name: "CoreDNS"
  annotations:
    k8s-addon: base
spec:
  selector:
    k8s-app: coredns
  clusterIP: 8.8.8.8
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
`)

var servicePatch = []byte(`
apiVersion: v1
kind: Service
metadata:
  name: coredns
  namespace: kube-system
spec:
  clusterIP: 1.2.3.4
`)

func TestStandalone(t *testing.T) {
	ffs := fs.MakeFakeFS()

	ffs.WriteFile("/kustomization.yaml", []byte(`
resources:
- service.yaml
`))

	ffs.WriteFile("/service.yaml", service)

	b, err := Build("/", loader.NewFileLoader(ffs))
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	var expected = buildExpected("8.8.8.8", "base")

	actual := strings.TrimSpace(string(b))
	if actual != expected {
		t.Errorf("unexpected result from Build: %s", diff.FormatDiff(expected, actual))
	}
}

func TestOverlay(t *testing.T) {
	ffs := fs.MakeFakeFS()

	ffs.WriteFile("/kustomization.yaml", []byte(`
resources:
- service.yaml
patches:
- patch.yaml
commonAnnotations:
  k8s-addon: overlay
`))

	ffs.WriteFile("/service.yaml", service)
	ffs.WriteFile("/patch.yaml", servicePatch)

	b, err := Build("/", loader.NewFileLoader(ffs))
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	expected := buildExpected("1.2.3.4", "overlay")

	actual := strings.TrimSpace(string(b))
	if actual != expected {
		t.Errorf("unexpected result from Build: %s", diff.FormatDiff(expected, actual))
	}
}

func TestBaseWithOverlay(t *testing.T) {
	ffs := fs.MakeFakeFS()

	ffs.WriteFile("/base/kustomization.yaml", []byte(`
resources:
- service.yaml
commonAnnotations:
  k8s-addon: overlay2
`))

	ffs.WriteFile("/base/service.yaml", service)

	ffs.WriteFile("/overlay/kustomization.yaml", []byte(`
bases:
- ../base/
patches:
- patch.yaml
`))

	ffs.WriteFile("/overlay/patch.yaml", servicePatch)

	b, err := Build("/overlay", loader.NewFileLoader(ffs))
	if err != nil {
		t.Fatalf("unexpected error from Build: %v", err)
	}

	expected := buildExpected("1.2.3.4", "overlay2")

	actual := strings.TrimSpace(string(b))
	if actual != expected {
		t.Errorf("unexpected result from Build: %s", diff.FormatDiff(expected, actual))
	}
}

func buildExpected(clusterIP string, k8sAddon string) string {
	expected := strings.TrimSpace(`
apiVersion: v1
kind: Service
metadata:
  annotations:
    k8s-addon: {{k8sAddon}}
  labels:
    addonmanager.kubernetes.io/mode: Reconcile
    k8s-app: coredns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: CoreDNS
  name: coredns
  namespace: kube-system
spec:
  clusterIP: {{clusterIP}}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
  selector:
    k8s-app: coredns
`)

	expected = strings.Replace(expected, "{{clusterIP}}", clusterIP, -1)
	expected = strings.Replace(expected, "{{k8sAddon}}", k8sAddon, -1)

	return expected
}
