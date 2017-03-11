package static

import "k8s.io/kops/tools/hostdisco/pkg/seeds"

type StaticSeedDiscovery struct {
	seeds []string
}

func New(seeds []string) *StaticSeedDiscovery {
	return &StaticSeedDiscovery{
		seeds: seeds,
	}
}

var _ seeds.SeedDiscovery = &StaticSeedDiscovery{}

func (s *StaticSeedDiscovery) GetSeeds() ([]string, error) {
	return s.seeds, nil
}