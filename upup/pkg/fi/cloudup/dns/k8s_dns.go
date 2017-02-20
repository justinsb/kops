package dns

//
//import (
//	"fmt"
//	"github.com/golang/glog"
//	"k8s.io/kubernetes/federation/pkg/dnsprovider"
//	"k8s.io/kubernetes/federation/pkg/dnsprovider/providers/google/clouddns"
//	"strings"
//)
//
//type KubernetesDNS struct {
//	Provider *clouddns.Interface
//}
//
//var _ Provider = &KubernetesDNS{}
//
//func (c *KubernetesDNS) FindDNSHostedZone(clusterDNSName string) (string, error) {
//	zonesProviders, supported := c.Provider.Zones()
//	if !supported {
//		return "", fmt.Errorf("DNS zone listing not supported by provider")
//	}
//
//	glog.V(2).Infof("Querying for all dns zones to find match for %q", clusterDNSName)
//	clusterDNSName = "." + strings.TrimSuffix(clusterDNSName, ".")
//
//	allZones, err := zonesProviders.List()
//	if err != nil {
//		return "", fmt.Errorf("error listing DNS zones: %v", err)
//	}
//
//	var zones []dnsprovider.Zone
//	for _, zone := range allZones {
//		zoneName := zone.Name()
//		zoneName = "." + strings.TrimSuffix(zoneName, ".")
//
//		if strings.HasSuffix(clusterDNSName, zoneName) {
//			zones = append(zones, zone)
//		}
//	}
//
//	// Find the longest zones
//	maxLength := -1
//	maxLengthZones := []dnsprovider.Zone{}
//	for _, z := range zones {
//		n := len(z.Name())
//		if n < maxLength {
//			continue
//		}
//
//		if n > maxLength {
//			maxLength = n
//			maxLengthZones = []dnsprovider.Zone{}
//		}
//
//		maxLengthZones = append(maxLengthZones, z)
//	}
//
//	if len(maxLengthZones) == 0 {
//		// We make this an error because you have to set up DNS delegation anyway
//		tokens := strings.Split(clusterDNSName, ".")
//		suffix := strings.Join(tokens[len(tokens)-2:], ".")
//		//glog.Warningf("No matching hosted zones found; will created %q", suffix)
//		//return suffix, nil
//		return "", fmt.Errorf("No matching hosted zones found for %q; please create one (e.g. %q) first", clusterDNSName, suffix)
//	}
//
//	if len(maxLengthZones) == 1 {
//		// TODO: Name vs id..
//		id := maxLengthZones[0].Name()
//		return id, nil
//	}
//
//	return "", fmt.Errorf("Found multiple hosted zones matching cluster %q; please specify the ID of the zone to use")
//}
