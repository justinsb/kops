package provider

//
//import (
//	"github.com/golang/glog"
//	"k8s.io/kops/protokube/pkg/gossip"
//	"fmt"
//	"k8s.io/kubernetes/bazel-kubernetes/external/io_bazel_rules_go_toolchain/src/strings"
//	"k8s.io/kubernetes/federation/pkg/dnsprovider/rrstype"
//)
//
//type GossipDnsStore struct {
//	dnsView gossip.DNSView
//}
//var _ Store = &GossipDnsStore{}
//
//
//func NewGossipStorage(gossipName string, gossipListen string, gossipSeeds gossip.SeedProvider) Store {
//	gossipState := &gossip.GossipState{
//		Listen: gossipListen,
//		Name:   gossipName,
//		Seeds:  gossipSeeds,
//	}
//	go func() {
//		// TODO: Cleanup
//		err := gossipState.Run()
//		if err != nil {
//			glog.Fatalf("gossip exited unexpectedly: %v", err)
//		} else {
//			glog.Fatalf("gossip exited unexpectedly, but without error")
//		}
//	}()
//
//	dnsView := gossip.NewDNSView(gossipState)
//
//	s := &GossipDnsStore{
//		dnsView: dnsView,
//	}
//	return s
//}
//
//
//func (s *GossipDnsStore) ListZones() ([]string, error) {
//	// TODO: The name isn't quite right
//	// TODO: Support multiple names?
//	zones := []string{ "gossip.local "}
//	return zones, nil
//}
//
//func (s *GossipDnsStore) QueryZoneRecords(zoneID string) ([]*ResourceRecordSet, error) {
//	var rrs []*ResourceRecordSet
//	suffix := "." + strings.TrimPrefix(zoneID, ".")
//
//	snapshot :=  s.dnsView.Snapshot()
//	for host, addresses:= range hostToAddr {
//		if !strings.HasSuffix(host, suffix) {
//			continue
//		} else {
//			// TODO: We really need richer records here
//			defaultTTL := 60
//			rrsType := rrstype.A
//			r := &ResourceRecordSet{
//				data: resourceRecordSetData{
//					Name: host,
//					Rrdatas: addresses,
//					Ttl : defaultTTL,
//					RrsType: rrsType,
//				},
//			}
//			rrs = append(rrs, r)
//		}
//	}
//
//}
//func (s *GossipDnsStore) UpdateZone(zone *Zone) error {
//
//}
//func (s *GossipDnsStore) RemoveZone(zone *Zone) error {
//	return fmt.Errorf("RemoveZone not supported for gossip")
//}
