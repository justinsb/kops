package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/go-openapi/spec"
	"k8s.io/kops/pkg/apis/kops/v1alpha2"
	"k8s.io/kube-openapi/pkg/common"
)

// Generate swagger from OpenAPI
// Inspired by https://github.com/argoproj/argo/blob/master/hack/gen-openapi-spec/main.go

// Generate OpenAPI spec definitions for Workflow Resource
func main() {
	oAPIDefs := v1alpha2.GetOpenAPIDefinitions(func(name string) spec.Ref {
		return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(normalize(name)))
	})
	defs := spec.Definitions{}
	for defName, val := range oAPIDefs {
		defs[normalize(defName)] = val.Schema
	}
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger:     "2.0",
			Definitions: defs,
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:   "Kops",
					Version: "v1alpha2",
				},
			},
		},
	}
	jsonBytes, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Println(string(jsonBytes))
}

// normalize remaps a go namespace to something more swagger-safe
func normalize(name string) string {
	// e.g. "io.k8s.apimachinery.pkg.apis.meta.v1.Status"

	name = strings.Replace(name, "k8s.io/kops", "kops.k8s.io", -1)
	parts := strings.Split(name, "/")
	hostParts := strings.Split(parts[0], ".")
	// reverses something like k8s.io to io.k8s
	for i, j := 0, len(hostParts)-1; i < j; i, j = i+1, j-1 {
		hostParts[i], hostParts[j] = hostParts[j], hostParts[i]
	}
	parts[0] = strings.Join(hostParts, ".")
	return strings.Join(parts, ".")
}
