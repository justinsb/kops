package seeds

type SeedDiscovery interface {
	GetSeeds() ([]string, error)
}


