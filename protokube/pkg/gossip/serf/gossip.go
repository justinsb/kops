package serf

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"k8s.io/kops/protokube/pkg/gossip"
	"net"
	"strconv"
	"sync"
	"time"
)

type SerfGossiper struct {
	Listen string
	Name   string
	Seeds  gossip.SeedProvider

	updateTagsMutex sync.Mutex
	tags            map[string]string

	mutex sync.Mutex
	serf  *serf.Serf

	peers map[string]*peer

	version uint64

	lastSnapshot *gossip.GossipStateSnapshot
}

var _ gossip.GossipState = &SerfGossiper{}

// peer represents the records published by a peer.  It is immutable, so the fields are public.
type peer struct {
	Id string

	Tags    map[string]string
	Healthy bool

	Version uint64
}

func (g *SerfGossiper) LatestVersion() uint64 {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.version
}

func (g *SerfGossiper) Snapshot() *gossip.GossipStateSnapshot {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if g.lastSnapshot != nil && g.lastSnapshot.Version == g.version {
		return g.lastSnapshot
	}
	values := make(map[string]string)
	for _, peer := range g.peers {
		if !peer.Healthy {
			// TODO: We might want to keep alive old records for a bit, in case serf fails but the machine hasn't
			// Also, we might want the concept of exclusive vs shared records
			continue
		}

		for k, v := range peer.Tags {
			if values[k] != "" {
				glog.Infof("found conflicting record %s=%s", k, v)
			}
			values[k] = v
		}
	}

	snapshot := &gossip.GossipStateSnapshot{
		Values:  values,
		Version: g.version,
	}
	g.lastSnapshot = snapshot
	return snapshot
}

func (g *SerfGossiper) Run() error {
	g.tags = make(map[string]string)
	g.peers = make(map[string]*peer)

	serfConfig := serf.DefaultConfig()

	{
		host, portString, err := net.SplitHostPort(g.Listen)
		if err != nil {
			return fmt.Errorf("cannot parse -listen flag: %v", g.Listen)
		}
		port, err := strconv.Atoi(portString)
		if err != nil {
			return fmt.Errorf("cannot parse -listen flag: %v", g.Listen)
		}
		serfConfig.MemberlistConfig.BindPort = port
		serfConfig.MemberlistConfig.AdvertisePort = port

		if host != "" {
			serfConfig.MemberlistConfig.BindAddr = host
		}
	}

	// Create a channel to listen for events from Serf
	eventCh := make(chan serf.Event, 64)
	serfConfig.EventCh = eventCh

	serfConfig.NodeName = g.Name

	serfConfig.Tags = g.tags

	s, err := serf.Create(serfConfig)
	if err != nil {
		return fmt.Errorf("error creating serf: %v", err)
	}

	g.serf = s

	// TODO: Merge into the loop here?
	go g.runSeeding()

	serfShutdownCh := s.ShutdownCh()
	for {
		select {
		case e := <-eventCh:
			switch e := e.(type) {
			case serf.MemberEvent:
				glog.Infof("%s: %v", e.Type, e.Members)

				g.updateMembers(e.Type, &e)

			default:
				glog.Warningf("unkonwn event type: %s", e.String())
			}

		case <-serfShutdownCh:
			glog.Infof("serf shutdown detected; quitting")
			return fmt.Errorf("serf shutdown detected")
		}
	}

	return nil
}

func (g *SerfGossiper) runSeeding() {
	for {
		glog.Infof("Querying for seeds")

		seeds, err := g.Seeds.GetSeeds()
		if err != nil {
			glog.Warningf("error getting seeds: %v", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		glog.Infof("Got seeds: %s", seeds)
		// TODO: Include ourselves?  Exclude ourselves?

		ignoreOld := false
		_, err = g.serf.Join(seeds, ignoreOld)
		if err != nil {
			glog.Warningf("error joining existing seeds %s: %v", seeds, err)
			time.Sleep(1 * time.Minute)
			continue
		}

		glog.Infof("Seeding successful")

		// Reseed periodically, just in case of partitions
		// TODO: Make it so that only one node polls, or at least statistically get close
		time.Sleep(60 * time.Minute)
	}
}

func (g *SerfGossiper) updateMembers(eventType serf.EventType, e *serf.MemberEvent) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.version++
	version := g.version

	for i := range e.Members {
		member := &e.Members[i]

		id := member.Name
		existing := g.peers[id]
		replacement := newPeer(id, version, member, existing)
		g.peers[id] = replacement
	}
}

func (g *SerfGossiper) UpdateValues(removeKeys []string, putEntries map[string]string) error {
	g.updateTagsMutex.Lock()
	defer g.updateTagsMutex.Unlock()

	newTags := make(map[string]string)
	for k, v := range g.tags {
		newTags[k] = v
	}
	for _, removeTag := range removeKeys {
		delete(newTags, removeTag)
	}
	for k, v := range putEntries {
		newTags[k] = v
	}
	err := g.serf.SetTags(newTags)
	if err == nil {
		g.tags = newTags
	}
	return err
}

func newPeer(id string, version uint64, member *serf.Member, previous *peer) *peer {
	p := &peer{Id: id, Version: version}
	switch member.Status {
	case serf.StatusAlive:
		if previous != nil && !previous.Healthy {
			glog.Infof("Node %q transitioning to healthy: %s", p.Id, member.Status)
		}
		p.Healthy = true
	case serf.StatusLeaving, serf.StatusLeft, serf.StatusFailed:
		if previous != nil && previous.Healthy {
			glog.Infof("Node %q transitioning to non-healthy: %s", p.Id, member.Status)
		}
		p.Healthy = false
	default:
		glog.Warningf("unknown status for peer %q: %v", p.Id, member.Status)
	}

	tags := make(map[string]string)
	for k, v := range member.Tags {
		tags[k] = v
	}
	p.Tags = tags
	return p
}
