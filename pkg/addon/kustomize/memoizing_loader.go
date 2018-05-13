package kustomize

import (
	"strings"

	"k8s.io/kubectl/pkg/loader"
)

type File struct {
	Path     string
	Contents []byte
}
type memoizingLoader struct {
	inner  loader.Loader
	parent *memoizingLoader
	Files  []File
}

var _ loader.Loader = &memoizingLoader{}

func NewMemoizingLoader(inner loader.Loader) *memoizingLoader {
	return &memoizingLoader{inner: inner}
}

func (l *memoizingLoader) Root() string {
	return l.inner.Root()
}

func (l *memoizingLoader) New(newRoot string) (loader.Loader, error) {
	newInner, err := l.inner.New(newRoot)
	if err != nil {
		return nil, err
	}

	return &memoizingLoader{inner: newInner, parent: l}, nil
}

func (l *memoizingLoader) Load(location string) ([]byte, error) {
	b, err := l.inner.Load(location)
	if err != nil {
		return nil, err
	}

	root := l
	for root.parent != nil {
		root = root.parent
	}

	fullPath := l.Root()
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += location

	root.Files = append(root.Files, File{Path: fullPath, Contents: b})

	return b, nil
}
