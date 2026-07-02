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

package instancegroups

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
)

func TestIsAPIServerUnreachable(t *testing.T) {
	connErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	wrapped := fmt.Errorf(`getting node "node": Get "https://example/api/v1/nodes/node": %w`, &url.Error{Op: "Get", URL: "https://example/api/v1/nodes/node", Err: connErr})

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "connection refused", err: wrapped, want: true},
		{name: "other error", err: errors.New("forbidden"), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAPIServerUnreachable(tc.err); got != tc.want {
				t.Fatalf("isAPIServerUnreachable() = %v, want %v", got, tc.want)
			}
		})
	}
}
