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

package cidr

import (
	"fmt"
	"net"
	"strings"
)

// Set contains a list of CIDRs
type Set struct {
	cidrs []net.IPNet
}

// NewSet builds a cidr.Set, parsing the strings
func NewSet(cidrs []string) (Set, error) {
	var ipNets []net.IPNet
	for _, s := range cidrs {
		_, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return Set{}, fmt.Errorf("CIDR %q not valid: %w", s, err)
		}
		ipNets = append(ipNets, *ipNet)
	}
	return Set{cidrs: ipNets}, nil
}

// WhereIPV4 returns a CIDRSet containing only the IPv4 CIDRs
func (s Set) WhereIPV4() Set {
	var matching []net.IPNet
	for i := range s.cidrs {
		if s.cidrs[i].IP.To4() != nil {
			matching = append(matching, s.cidrs[i])
		}
	}
	return Set{cidrs: matching}
}

// WhereIPV6 returns a CIDRSet containing only the IPv6 CIDRs
func (s Set) WhereIPV6() Set {
	var matching []net.IPNet
	for i := range s.cidrs {
		if s.cidrs[i].IP.To4() == nil {
			matching = append(matching, s.cidrs[i])
		}
	}
	return Set{cidrs: matching}
}

// ToStrings returns the CIDRs as a list of strings.
func (s Set) ToStrings() []string {
	var ret []string
	for i := range s.cidrs {
		ret = append(ret, s.cidrs[i].String())
	}
	return ret
}

// Len returns the number of CIDRs in the Set.
func (s Set) Len() int {
	return len(s.cidrs)
}

// String formats the CIDRSet for printing.
func (s Set) String() string {
	var b strings.Builder
	for i := range s.cidrs {
		if i != 0 {
			b.WriteString(",")
		}
		b.WriteString(s.cidrs[i].String())
	}
	return b.String()
}
