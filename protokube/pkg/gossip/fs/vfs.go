/*
Copyright 2017 The Kubernetes Authors.

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

package fs

import (
	"k8s.io/kops/util/pkg/vfs"
	"os"
	"path"
)

type GossipFSPath struct {
	view *FSView
	path string
}

var _ vfs.Path = &GossipFSPath{}

type GossipFSContext struct {
	root *GossipFSPath
}

func NewGossipFSContext(view *FSView) *GossipFSContext {
	c := &GossipFSContext{}
	c.root = &GossipFSPath{
		view: view,
		path: "/",
	}
	return c
}

func (p *GossipFSPath) IsClusterReadable() bool {
	return true
}

var _ vfs.HasClusterReadable = &GossipFSPath{}

func NewGossipFSPath(context *GossipFSContext, location string) *GossipFSPath {
	return context.root.Join(location).(*GossipFSPath)
}

func (p *GossipFSPath) Join(relativePath ...string) vfs.Path {
	args := []string{p.path}
	args = append(args, relativePath...)
	joined := path.Join(args...)
	return &GossipFSPath{
		view: p.view,
		path: joined,
	}
}

func (p *GossipFSPath) WriteFile(data []byte) error {
	return p.view.PutFile(p.path, data)
}

func (p *GossipFSPath) CreateFile(data []byte) error {
	// Check if exists
	if p.view.Snapshot().File(p.path) != nil {
		return os.ErrExist
	}

	return p.WriteFile(data)
}

func (p *GossipFSPath) ReadFile() ([]byte, error) {
	f := p.view.Snapshot().File(p.path)
	if f == nil {
		return nil, os.ErrNotExist
	}
	// TODO: Copy?
	return f.Data, nil
}

func (p *GossipFSPath) ReadDir() ([]vfs.Path, error) {
	var paths []vfs.Path
	for k := range p.view.Snapshot().Children(p.path) {
		paths = append(paths, &GossipFSPath{view:p.view, path: k})
	}
	return paths, nil
}

func (p *GossipFSPath) ReadTree() ([]vfs.Path, error) {
	var paths []vfs.Path
	for k := range p.view.Snapshot().ReadTree(p.path) {
		paths = append(paths, &GossipFSPath{view:p.view, path: k})
	}
	return paths, nil
}

func (p *GossipFSPath) Base() string {
	return path.Base(p.path)
}

func (p *GossipFSPath) Path() string {
	return "gossipfs:/" + p.path
}

func (p *GossipFSPath) String() string {
	return p.Path()
}

func (p *GossipFSPath) Remove() error {
	return p.view.DeleteFile(p.path)
}
