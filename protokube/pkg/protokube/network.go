package protokube

import (
	"fmt"
	"github.com/golang/glog"
	"net"
	"strings"
)

func findEthernetIps() ([]net.IP, error) {
	var ips []net.IP

	networkInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("error querying interfaces to determine internal ip: %v", err)
	}

	for i := range networkInterfaces {
		networkInterface := &networkInterfaces[i]
		flags := networkInterface.Flags
		name := networkInterface.Name

		if (flags & net.FlagLoopback) != 0 {
			glog.V(2).Infof("Ignoring interface %s - loopback", name)
			continue
		}

		// Not a lot else to go on...
		// eth are the "traditional" names
		// Ubuntu 15.10 introduces names that start with "en"
		if !strings.HasPrefix(name, "eth") && !strings.HasPrefix(name, "en") {
			glog.V(2).Infof("Ignoring interface %s - name does not look like ethernet device", name)
			continue
		}

		addrs, err := networkInterface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("error querying network interface %s for IP adddresses: %v", name, err)
		}

		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil {
				return nil, fmt.Errorf("error parsing address %s on network interface %s: %v", addr.String(), name, err)
			}

			if ip.IsLoopback() {
				glog.V(2).Infof("Ignoring address %s (loopback)", ip)
				continue
			}

			if ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
				glog.V(2).Infof("Ignoring address %s (link-local)", ip)
				continue
			}

			ips = append(ips, ip)
		}
	}

	return ips, nil
}

// Returns the internal IP address of this machine (which may also be the external IP)
func FindInternalIP() (net.IP, error) {
	ips, err := findEthernetIps()
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("unable to determine internal ip (no adddresses found)")
	}

	if len(ips) != 1 {
		// TODO: Prefer non-external IPs?
		glog.Warningf("Found multiple internal IPs; making arbitrary choice")
		for _, ip := range ips {
			glog.Warningf("\tip: %s", ip.String())
		}
	} else {
		glog.Infof("Determined internal IP to be %v", ips[0])
	}
	return ips[0], nil
}

// Returns the external IP address of this machine
func FindExternalIPs() ([]net.IP, error) {
	ips, err := findEthernetIps()
	if err != nil {
		return nil, err
	}

	var external []net.IP
	for _, ip := range ips {
		if isPrivate(ip) {
			continue
		}
		external = append(external, ip)
	}

	return external, nil
}

func isPrivate(ip net.IP) bool {
	for _, privateCIDR := range []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"224.0.0.0/4"} {
		_, cidr, err := net.ParseCIDR(privateCIDR)
		if err != nil {
			glog.Fatalf("unexpectedly failed to parse CIDR %q", privateCIDR)
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
