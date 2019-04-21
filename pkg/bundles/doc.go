// Package v1alpha1 represents the v1alpha1 version of the Cluster Bundle API.
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +groupName=bundle.gke.io
package bundles

// Generate deepcopy for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./... -h ../../hack/boilerplate/boilerplate.go.txt
