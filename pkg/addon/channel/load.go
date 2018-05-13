package channel

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/golang/glog"
	"k8s.io/kops/upup/pkg/fi/utils"
	"k8s.io/kops/util/pkg/vfs"
)

type Channel struct {
	p       vfs.Path
	channel *ChannelSpec
}

func LoadAddonChannel(p vfs.Path) (*Channel, error) {
	glog.V(2).Infof("Loading addon channel from %q", p)
	data, err := p.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("error reading addons from %q: %v", p, err)
	}

	return ParseAddonChannel(p, data)
}

func ParseAddonChannel(p vfs.Path, data []byte) (*Channel, error) {
	// Yaml can't parse empty strings
	s := string(data)
	s = strings.TrimSpace(s)

	o := &ChannelSpec{}
	if s != "" {
		err := utils.YamlUnmarshal([]byte(s), o)
		if err != nil {
			return nil, fmt.Errorf("error parsing addons: %v", err)
		}
	}

	return &Channel{p: p, channel: o}, nil
}

type Criteria struct {
	// Name string
}

func (c *Criteria) Matches(a *ChannelVersion) bool {
	// if c.Name != "" && c.Name != a.Name {
	// 	return false
	// }
	return true
}

func (a *Channel) FindMatches(c *Criteria) []*ChannelVersion {
	var addons []*ChannelVersion
	for _, addon := range a.channel.Versions {
		if c.Matches(addon) {
			addons = append(addons, addon)
		}
	}
	return addons
}

func (a *Channel) BestMatch(c *Criteria) *ChannelVersion {
	matches := a.FindMatches(c)

	if len(matches) == 0 {
		return nil
	}

	var best *ChannelVersion
	var bestVersion semver.Version

	for _, addon := range matches {
		addonVersion, err := semver.ParseTolerant(addon.Version)
		if err != nil {
			glog.Warningf("unable to parse version from %v", addon)
			continue
		}

		if best == nil || addonVersion.GTE(bestVersion) {
			best = addon
			bestVersion = addonVersion
		}
	}

	return best
}
