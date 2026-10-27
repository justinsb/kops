/*
Copyright 2026 The Kubernetes Authors.

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

package sandbox

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func fakeProbe(goos string, onPath map[string]bool, files map[string]bool) hostProbe {
	return hostProbe{
		goos: goos,
		lookPath: func(name string) (string, error) {
			if onPath[name] {
				return "/fake/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		stat: func(p string) (os.FileInfo, error) {
			if files[p] {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
	}
}

func TestDetect(t *testing.T) {
	all := map[string]bool{"container": true, "docker": true}
	seatbelt := map[string]bool{seatbeltBinary: true}
	cases := []struct {
		name      string
		requested string
		probe     hostProbe
		want      string
		wantErr   bool
	}{
		{"darwin prefers container", "", fakeProbe("darwin", all, seatbelt), NameContainer, false},
		{"darwin falls back to docker", "", fakeProbe("darwin", map[string]bool{"docker": true}, seatbelt), NameDocker, false},
		{"darwin falls back to seatbelt", "", fakeProbe("darwin", nil, seatbelt), NameSeatbelt, false},
		{"darwin with nothing", "", fakeProbe("darwin", nil, nil), NameNone, false},
		{"linux prefers docker", "", fakeProbe("linux", all, nil), NameDocker, false},
		{"linux with nothing", "", fakeProbe("linux", nil, nil), NameNone, false},
		{"explicit seatbelt", "seatbelt", fakeProbe("darwin", all, seatbelt), NameSeatbelt, false},
		{"explicit none", "none", fakeProbe("darwin", all, seatbelt), NameNone, false},
		{"explicit docker missing", "docker", fakeProbe("linux", nil, nil), "", true},
		{"container on linux", "container", fakeProbe("linux", all, nil), "", true},
		{"unknown name", "firejail", fakeProbe("linux", all, nil), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, err := detect(tc.requested, tc.probe)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %s", e.Name())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.Name() != tc.want {
				t.Errorf("got %s, want %s", e.Name(), tc.want)
			}
		})
	}
}

func TestSeatbeltProfile(t *testing.T) {
	cmd := &Command{
		Tool:   Tool{Name: "helm", HostPath: "/Users/me/.kops/tools/helm/v3/helm"},
		Mounts: []Mount{{Path: "/private/tmp/work/chart", ReadOnly: true}, {Path: "/private/tmp/work/out"}},
	}
	p := seatbeltProfile(cmd)
	for _, want := range []string{
		"(deny default)",
		`(allow process-exec (literal "/Users/me/.kops/tools/helm/v3/helm"))`,
		`(allow file-read* (subpath "/private/tmp/work/chart"))`,
		`(allow file-read* file-write* (subpath "/private/tmp/work/out"))`,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("profile missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "(allow network*)") {
		t.Errorf("network must be denied by default:\n%s", p)
	}
	cmd.Network = true
	if !strings.Contains(seatbeltProfile(cmd), "(allow network*)") {
		t.Errorf("network should be allowed when requested")
	}
	if got := sbString(`a"b\c`); got != `"a\"b\\c"` {
		t.Errorf("sbString = %s", got)
	}
}

func TestDropDeprecation(t *testing.T) {
	var out strings.Builder
	w := &dropDeprecation{w: &out}
	w.Write([]byte("2026-10-27 sandbox-exec[1]: sandbox-exec is deprecated\nreal error\n"))
	if out.String() != "real error\n" {
		t.Errorf("got %q", out.String())
	}
}
