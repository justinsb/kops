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
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kops/protokube/pkg/gossip"
	"strings"
	"sync"
	"time"
	"github.com/gogo/protobuf/proto"
)

type FSView struct {
	gossipState gossip.GossipState

	mutex        sync.Mutex
	lastSnapshot *FSViewSnapshot
}

type FSViewSnapshot struct {
	version uint64
	fileMap map[string]*FSEntry
}

// File returns the matching file, if it is found
func (s *FSViewSnapshot) File(key string) *FSEntry {
	return s.fileMap[key]
}

// WithPrefix returns the children with the specified path prefix (i.e. recursive children)
func (s *FSViewSnapshot) WithPrefix(prefix string) map[string]*FSEntry {
	matches := make(map[string]*FSEntry)
	for k, v := range s.fileMap {
		if strings.HasPrefix(k, prefix) {
			matches[k] = v
		}
	}
	return matches
}


// Children returns the direct (non-recursive) children of the specified parent
func (s *FSViewSnapshot) Children(parent string) map[string]*FSEntry {
	if !strings.HasSuffix(parent, "/") {
		parent = parent + "/"
	}
	matches := make(map[string]*FSEntry)
	for k, v := range s.fileMap {
		if strings.HasPrefix(k, parent) {
			relativePath := strings.TrimPrefix(k, parent)
			if strings.Index(relativePath, "/") == -1 {
				matches[k] = v
			}
		}
	}
	return matches
}

// PutFile creates or updates the specified file
func (v *FSView) PutFile(key string, entry *FSEntry) error {
	var removeTags []string

	b, err := proto.Marshal(entry)
	if err != nil {
		return fmt.Errorf("error serializing FSEntry: %v", err)
	}

	createTags := make(map[string][]byte)
	createTags["fs/" + key] = b

	return v.gossipState.UpdateValues(removeTags, createTags)
}

// DeleteFile deletes the specified file.
func (v *FSView) DeleteFile(key string) error {
	removeTags := []string{"fs/" + key}
	createTags := make(map[string][]byte)

	return v.gossipState.UpdateValues(removeTags, createTags)
}

func NewFSView(gossipState gossip.GossipState) *FSView {
	return &FSView{
		gossipState: gossipState,
	}
}

// Snapshot returns a copy of the current desired DNS state-of-the-world
func (v *FSView) Snapshot() *FSViewSnapshot {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	gossipSnapshot := v.gossipState.Snapshot()
	// Snapshot must be cheap if nothing has changed
	if v.lastSnapshot != nil && gossipSnapshot.Version == v.lastSnapshot.version {
		return v.lastSnapshot
	}

	snapshot := &FSViewSnapshot{
		version: gossipSnapshot.Version,
	}

	fileMap := make(map[string]*FSEntry)
	for k, v := range gossipSnapshot.Values {
		if strings.HasPrefix(k, "fs /") {
			fsEntry := &FSEntry{}
			err := proto.Unmarshal(v)
			if err != nil {
				glog.Warningf("ignoring corrupted file %q: %v", k, err)
				continue
			}
			fileMap[k[2:]] = fsEntry
		} else if !strings.HasPrefix(k, "dns /") {
			glog.Warningf("unknown tag %q=%q", k, v)
		}
	}

	snapshot.fileMap = fileMap
	v.lastSnapshot = snapshot

	return snapshot
}
