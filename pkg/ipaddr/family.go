package ipaddr

import (
	"fmt"
	"net"
	"strings"
)

// Family is the IP address family (type) for an address; typically IPV4 or IPV6
type Family string

const (
	FamilyIPV4 Family = "ipv4"
	FamilyIPV6 Family = "ipv6"
)

// GetFamily returns the family for an IP.
// IPv4-in-IPv6 addresses are treated as IPv6.
func GetFamily(ip string) (Family, error) {
	// Verify it is an IP (and not a host:port)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("cannot parse IP %q", ip)
	}

	// Check for ipv6 using colons; this is hacky but means we classify IPv4-in-IPv6 as v6
	// Context: https://github.com/golang/go/issues/37921
	family := FamilyIPV4
	if strings.Contains(ip, ":") {
		family = FamilyIPV6
	}

	return family, nil
}
