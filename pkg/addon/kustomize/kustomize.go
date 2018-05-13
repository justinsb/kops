package kustomize

import (
	"k8s.io/kubectl/pkg/kustomize/app"
	"k8s.io/kubectl/pkg/kustomize/resource"
	kutil "k8s.io/kubectl/pkg/kustomize/util"
	"k8s.io/kubectl/pkg/loader"
)

func Walk(kustomizationPath string, loaders ...loader.SchemeLoader) ([]File, error) {
	l := loader.Init(loaders)

	ml := NewMemoizingLoader(l)

	_, err := build(kustomizationPath, ml)
	if err != nil {
		return nil, err
	}

	return ml.Files, nil
}

func BuildResources(kustomizationPath string, loaders ...loader.SchemeLoader) (resource.ResourceCollection, error) {
	l := loader.Init(loaders)

	// absPath, err := filepath.Abs(kustomizationPath)
	// if err != nil {
	// 	return nil, err
	// }

	return build(kustomizationPath, l)
}

func Build(kustomizationPath string, loaders ...loader.SchemeLoader) ([]byte, error) {
	allResources, err := BuildResources(kustomizationPath, loaders...)
	if err != nil {
		return nil, err
	}

	// Output the objects.
	res, err := kutil.Encode(allResources)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func build(kustomizationPath string, loader loader.Loader) (resource.ResourceCollection, error) {
	rootLoader, err := loader.New(kustomizationPath)
	if err != nil {
		return nil, err
	}

	application, err := app.New(rootLoader)
	if err != nil {
		return nil, err
	}

	allResources, err := application.Resources()
	if err != nil {
		return nil, err
	}

	return allResources, nil
}
