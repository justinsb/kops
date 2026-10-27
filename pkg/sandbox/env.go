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

import "os"

// minimalEnv builds the environment for a process-based executor: only PATH (so the tool can find
// nothing it should not), HOME pointing nowhere useful unless the caller sets it, and the caller's
// variables. The operator's credentials and tokens are deliberately not inherited.
func minimalEnv(extra map[string]string) map[string]string {
	env := map[string]string{"PATH": "/usr/bin:/bin"}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

// hostEnv returns the full host environment as a map, for running the container runtime itself.
func hostEnv() map[string]string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				env[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return env
}
