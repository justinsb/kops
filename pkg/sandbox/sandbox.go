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

// Package sandbox runs external tools (today: helm) with restricted access to the host.
//
// kOps renders Helm charts on the operator's machine during `kops update cluster`. Chart templates are
// third-party code, so the renderer is run with no network and with only the working directory visible.
// The isolation mechanism is chosen with KOPS_SANDBOX or detected: Apple's `container` tool on macOS 26,
// Docker, Seatbelt (`sandbox-exec`) on macOS, or no isolation as a last resort.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"k8s.io/klog/v2"
)

// Tool identifies the program to run. Process-based executors run HostPath; image-based executors run
// the tool's entrypoint inside Image.
type Tool struct {
	// Name is the tool name, for messages.
	Name string
	// HostPath is the path of the tool binary on this machine.
	HostPath string
	// Image is a container image providing the tool, pinned by digest.
	Image string
}

// Mount makes a host directory visible to the tool at the same path.
type Mount struct {
	// Path is an absolute, symlink-free host directory path. It is visible at the same path inside
	// the sandbox, so callers can use host paths in Args with every executor.
	Path string
	// ReadOnly denies writes.
	ReadOnly bool
}

// Command is one tool invocation.
type Command struct {
	Tool Tool
	// Args are the arguments after the program name.
	Args []string
	// Env are environment variables set for the tool. Executors do not pass through the host environment.
	Env map[string]string
	// Dir is the working directory; it must be under one of the Mounts.
	Dir string
	// Mounts are the only host directories the tool can see.
	Mounts []Mount
	// Network allows outbound network access.
	Network bool
	Stdout  io.Writer
	Stderr  io.Writer
}

// Executor runs commands with some isolation mechanism.
type Executor interface {
	// Name is the executor name, as accepted by KOPS_SANDBOX.
	Name() string
	// Run runs the command and returns an error if it fails or exits non-zero.
	Run(ctx context.Context, cmd *Command) error
}

// EnvVar is the environment variable that selects an executor.
const EnvVar = "KOPS_SANDBOX"

// Names of the executors, in auto-detection order on macOS.
const (
	NameContainer = "container"
	NameDocker    = "docker"
	NameSeatbelt  = "seatbelt"
	NameNone      = "none"
)

// hostProbe abstracts the host checks so detection can be unit tested.
type hostProbe struct {
	goos     string
	lookPath func(string) (string, error)
	stat     func(string) (os.FileInfo, error)
}

func defaultProbe() hostProbe {
	return hostProbe{goos: runtime.GOOS, lookPath: exec.LookPath, stat: os.Stat}
}

// Detect returns the executor selected by KOPS_SANDBOX, or the best one available on this host.
func Detect() (Executor, error) {
	return detect(os.Getenv(EnvVar), defaultProbe())
}

func detect(requested string, probe hostProbe) (Executor, error) {
	if requested != "" {
		e, err := byName(strings.ToLower(strings.TrimSpace(requested)), probe)
		if err != nil {
			return nil, fmt.Errorf("%s=%q: %w", EnvVar, requested, err)
		}
		return e, nil
	}
	var candidates []string
	switch probe.goos {
	case "darwin":
		candidates = []string{NameContainer, NameDocker, NameSeatbelt}
	default:
		candidates = []string{NameDocker}
	}
	for _, name := range candidates {
		if e, err := byName(name, probe); err == nil {
			return e, nil
		}
	}
	klog.Warningf("no sandbox available for running external tools; set %s to choose one, or install Docker (or, on macOS, Apple's container tool). Running without isolation.", EnvVar)
	return &noneExecutor{}, nil
}

func byName(name string, probe hostProbe) (Executor, error) {
	switch name {
	case NameContainer:
		if probe.goos != "darwin" {
			return nil, fmt.Errorf("the container executor is only available on macOS")
		}
		p, err := probe.lookPath("container")
		if err != nil {
			return nil, fmt.Errorf("container tool not found on PATH")
		}
		return &imageExecutor{name: NameContainer, binary: p}, nil
	case NameDocker:
		p, err := probe.lookPath("docker")
		if err != nil {
			return nil, fmt.Errorf("docker not found on PATH")
		}
		return &imageExecutor{name: NameDocker, binary: p}, nil
	case NameSeatbelt:
		if probe.goos != "darwin" {
			return nil, fmt.Errorf("seatbelt is only available on macOS")
		}
		if _, err := probe.stat(seatbeltBinary); err != nil {
			return nil, fmt.Errorf("%s not found", seatbeltBinary)
		}
		return &seatbeltExecutor{}, nil
	case NameNone:
		return &noneExecutor{}, nil
	default:
		return nil, fmt.Errorf("unknown sandbox %q (valid: %s, %s, %s, %s)", name, NameContainer, NameDocker, NameSeatbelt, NameNone)
	}
}

// envSlice renders Env for exec.Cmd.
func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func runProcess(ctx context.Context, name string, argv []string, env map[string]string, dir string, stdout, stderr io.Writer) error {
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Env = envSlice(env)
	c.Dir = dir
	c.Stdout = stdout
	c.Stderr = stderr
	klog.V(2).Infof("sandbox %s: running %s", name, strings.Join(argv, " "))
	if err := c.Run(); err != nil {
		return fmt.Errorf("sandbox %s: %s failed: %w", name, argv[0], err)
	}
	return nil
}
