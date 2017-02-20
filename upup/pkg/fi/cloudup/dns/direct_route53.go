package dns

//
//import (
//	"fmt"
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/service/route53"
//	"github.com/golang/glog"
//	"strings"
//)
//
//type DirectRoute53DNS struct {
//	Route53 *route53.Route53
//}
//
//var _ Provider = &DirectRoute53DNS{}
//
//func (c *DirectRoute53DNS) FindDNSHostedZone(clusterDNSName string) (string, error) {
//	glog.V(2).Infof("Querying for all route53 zones to find match for %q", clusterDNSName)
//
//	clusterDNSName = "." + strings.TrimSuffix(clusterDNSName, ".")
//
//	var zones []*route53.HostedZone
//	request := &route53.ListHostedZonesInput{}
//	err := c.Route53.ListHostedZonesPages(request, func(p *route53.ListHostedZonesOutput, lastPage bool) bool {
//		for _, zone := range p.HostedZones {
//			zoneName := aws.StringValue(zone.Name)
//			zoneName = "." + strings.TrimSuffix(zoneName, ".")
//
//			if strings.HasSuffix(clusterDNSName, zoneName) {
//				zones = append(zones, zone)
//			}
//		}
//		return true
//	})
//	if err != nil {
//		return "", fmt.Errorf("error querying for route53 zones: %v", err)
//	}
//
//	// Find the longest zones
//	maxLength := -1
//	maxLengthZones := []*route53.HostedZone{}
//	for _, z := range zones {
//		n := len(aws.StringValue(z.Name))
//		if n < maxLength {
//			continue
//		}
//
//		if n > maxLength {
//			maxLength = n
//			maxLengthZones = []*route53.HostedZone{}
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
//		id := aws.StringValue(maxLengthZones[0].Id)
//		id = strings.TrimPrefix(id, "/hostedzone/")
//		return id, nil
//	}
//
//	return "", fmt.Errorf("Found multiple hosted zones matching cluster %q; please specify the ID of the zone to use")
//}
