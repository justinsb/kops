/*
Copyright 2019 The Kubernetes Authors.

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

package nodetasks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"k8s.io/kops/upup/pkg/fi"
)

func TestServiceTask_Deps(t *testing.T) {
	s := &Service{}

	tasks := make(map[string]fi.NodeupTask)
	tasks["LoadImageTask1"] = &LoadImageTask{}
	tasks["FileTask1"] = &File{}

	deps := s.GetDependencies(tasks)
	expected := []fi.NodeupTask{tasks["FileTask1"]}
	if !reflect.DeepEqual(expected, deps) {
		t.Fatalf("unexpected deps.  expected=%v, actual=%v", expected, deps)
	}
}

type FakeTask struct{}

func (t *FakeTask) Run(*fi.NodeupContext) error {
	panic("not implemented")
}

func TestServiceTask_UnknownTypes(t *testing.T) {
	s := &Service{}

	tasks := make(map[string]fi.NodeupTask)
	tasks["FakeTask1"] = &FakeTask{}

	deps := s.GetDependencies(tasks)
	expected := []fi.NodeupTask{tasks["FakeTask1"]}
	if !reflect.DeepEqual(expected, deps) {
		t.Fatalf("unexpected deps.  expected=%v, actual=%v", expected, deps)
	}
}

func TestNeedsSmartRestart(t *testing.T) {
	tempDir := t.TempDir()

	binaryPath := filepath.Join(tempDir, "kubelet")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}
	binaryModTime := time.Date(2026, time.July, 2, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(binaryPath, binaryModTime, binaryModTime); err != nil {
		t.Fatalf("setting binary mtime: %v", err)
	}

	definition := "[Service]\nExecStart=" + binaryPath + " \"$DAEMON_ARGS\"\n"

	// The systemd unit file itself; put it in tempDir so the mtime is under our control.
	unitPath := filepath.Join(tempDir, "kubelet.service")
	if err := os.WriteFile(unitPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("writing unit file: %v", err)
	}
	if err := os.Chtimes(unitPath, binaryModTime, binaryModTime); err != nil {
		t.Fatalf("setting unit file mtime: %v", err)
	}

	grid := []struct {
		name       string
		properties map[string]string
		want       bool
	}{
		{
			name:       "service started before binary was updated",
			properties: map[string]string{"ExecMainStartTimestamp": "Thu 2026-07-02 11:00:00 UTC"},
			want:       true,
		},
		{
			name:       "service started after binary was updated",
			properties: map[string]string{"ExecMainStartTimestamp": "Thu 2026-07-02 13:00:00 UTC"},
			want:       false,
		},
		{
			name:       "no start timestamp",
			properties: map[string]string{},
			want:       false,
		},
	}

	for _, g := range grid {
		t.Run(g.name, func(t *testing.T) {
			got, err := needsSmartRestart("kubelet.service", definition, tempDir, g.properties)
			if err != nil {
				t.Fatalf("needsSmartRestart failed: %v", err)
			}
			if got != g.want {
				t.Errorf("needsSmartRestart: got %v, want %v", got, g.want)
			}
		})
	}
}
