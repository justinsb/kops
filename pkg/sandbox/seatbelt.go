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
	"os"
	"path/filepath"
	"strings"
)

// seatbeltBinary is macOS's command-line interface to the Seatbelt sandbox.
const seatbeltBinary = "/usr/bin/sandbox-exec"

// seatbeltExecutor runs the host binary under a generated Seatbelt profile: no network unless asked,
// read access to the system libraries the binary needs, and read or write access to the Mounts only.
type seatbeltExecutor struct{}

func (e *seatbeltExecutor) Name() string { return NameSeatbelt }

func (e *seatbeltExecutor) Run(ctx context.Context, cmd *Command) error {
	if cmd.Tool.HostPath == "" {
		return fmt.Errorf("sandbox %s: tool %q has no host binary", NameSeatbelt, cmd.Tool.Name)
	}
	dir, err := os.MkdirTemp("", "kops-seatbelt-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	profilePath := filepath.Join(dir, "profile.sb")
	if err := os.WriteFile(profilePath, []byte(seatbeltProfile(cmd)), 0o600); err != nil {
		return err
	}
	argv := []string{seatbeltBinary, "-f", profilePath, cmd.Tool.HostPath}
	argv = append(argv, cmd.Args...)
	env := minimalEnv(cmd.Env)
	// sandbox-exec prints a deprecation notice on every run; it has done so for years.
	return runProcess(ctx, NameSeatbelt, argv, env, cmd.Dir, cmd.Stdout, &dropDeprecation{w: cmd.Stderr})
}

// seatbeltProfile renders the Seatbelt policy for a command.
func seatbeltProfile(cmd *Command) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(deny default)\n")
	// The binary itself, and the system paths any Go binary needs (dyld shared cache, libSystem,
	// time zones, random devices). Metadata reads are allowed everywhere so path resolution and
	// os.Getwd work; file contents stay restricted.
	fmt.Fprintf(&b, "(allow process-exec (literal %s))\n", sbString(cmd.Tool.HostPath))
	b.WriteString("(allow process-fork)\n")
	b.WriteString("(allow file-read-metadata)\n")
	fmt.Fprintf(&b, "(allow file-read* (literal %s))\n", sbString(cmd.Tool.HostPath))
	// Go binaries read the root directory itself during startup (path resolution of the executable);
	// without this helm exits 1 before printing anything. Only "/" is allowed, not its contents.
	b.WriteString("(allow file-read* (literal \"/\"))\n")
	for _, p := range []string{"/usr/lib", "/System", "/private/var/db/dyld", "/usr/share/zoneinfo", "/private/etc/ssl", "/etc/ssl", "/Library/Keychains", "/private/var/db/mds", "/System/Library/Keychains"} {
		fmt.Fprintf(&b, "(allow file-read* (subpath %s))\n", sbString(p))
	}
	for _, p := range []string{"/dev/null", "/dev/zero", "/dev/random", "/dev/urandom", "/dev/dtracehelper"} {
		fmt.Fprintf(&b, "(allow file-read* file-write* (literal %s))\n", sbString(p))
	}
	for _, m := range cmd.Mounts {
		if m.ReadOnly {
			fmt.Fprintf(&b, "(allow file-read* (subpath %s))\n", sbString(m.Path))
		} else {
			fmt.Fprintf(&b, "(allow file-read* file-write* (subpath %s))\n", sbString(m.Path))
		}
	}
	// Go runtime housekeeping: sysctl for CPU/version info, Mach services for certificate trust
	// evaluation (needed when Network is on), POSIX shared memory.
	b.WriteString("(allow sysctl-read)\n(allow mach-lookup)\n(allow ipc-posix-shm)\n")
	if cmd.Network {
		b.WriteString("(allow network*)\n")
	}
	return b.String()
}

// sbString quotes a string for a Seatbelt profile.
func sbString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// dropDeprecation filters sandbox-exec's deprecation notice out of stderr.
type dropDeprecation struct {
	w   interface{ Write([]byte) (int, error) }
	buf []byte
}

func (d *dropDeprecation) Write(p []byte) (int, error) {
	if d.w == nil {
		return len(p), nil
	}
	d.buf = append(d.buf, p...)
	for {
		i := strings.IndexByte(string(d.buf), '\n')
		if i < 0 {
			return len(p), nil
		}
		line := d.buf[:i+1]
		d.buf = d.buf[i+1:]
		if strings.Contains(string(line), "sandbox-exec") && strings.Contains(string(line), "deprecated") {
			continue
		}
		if _, err := d.w.Write(line); err != nil {
			return 0, err
		}
	}
}
