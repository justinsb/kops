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
	"context"
	"fmt"
)

// imageExecutor runs the tool's container image with Apple's `container` CLI or with Docker. Both
// accept the same `run` flags for what we need. Mounts are mapped to identical paths inside the
// container so that Args can use host paths.
type imageExecutor struct {
	name   string
	binary string
}

func (e *imageExecutor) Name() string { return e.name }

func (e *imageExecutor) Run(ctx context.Context, cmd *Command) error {
	if cmd.Tool.Image == "" {
		return fmt.Errorf("sandbox %s: tool %q has no container image", e.name, cmd.Tool.Name)
	}
	argv := []string{e.binary, "run", "--rm"}
	if !cmd.Network {
		argv = append(argv, "--network", "none")
	}
	for _, m := range cmd.Mounts {
		spec := m.Path + ":" + m.Path
		if m.ReadOnly {
			spec += ":ro"
		}
		argv = append(argv, "-v", spec)
	}
	if cmd.Dir != "" {
		argv = append(argv, "-w", cmd.Dir)
	}
	for k, v := range cmd.Env {
		argv = append(argv, "-e", k+"="+v)
	}
	argv = append(argv, cmd.Tool.Image)
	argv = append(argv, cmd.Args...)
	// The container runtime itself needs the host environment (e.g. to find its own daemon or
	// config), so only the tool inside the container runs with a scrubbed environment.
	return runProcess(ctx, e.name, argv, hostEnv(), "", cmd.Stdout, cmd.Stderr)
}

// noneExecutor runs the tool directly, with a scrubbed environment but no isolation.
type noneExecutor struct{}

func (e *noneExecutor) Name() string { return NameNone }

func (e *noneExecutor) Run(ctx context.Context, cmd *Command) error {
	if cmd.Tool.HostPath == "" {
		return fmt.Errorf("sandbox %s: tool %q has no host binary", NameNone, cmd.Tool.Name)
	}
	argv := append([]string{cmd.Tool.HostPath}, cmd.Args...)
	return runProcess(ctx, NameNone, argv, minimalEnv(cmd.Env), cmd.Dir, cmd.Stdout, cmd.Stderr)
}
