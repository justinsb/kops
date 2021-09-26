package gceresolver

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"google.golang.org/api/compute/v1"
	"k8s.io/klog"
)

type GCEResolver struct {
	compute   *compute.Service
	projectID string
	zoneNames []string
}

// Each page can have 500 results, but we cap how many pages
// are iterated through to prevent infinite loops if the API
// were to continuously return a nextPageToken.
const maxPages = 100

func (r *GCEResolver) Resolve(ctx context.Context, addr string) ([]string, error) {
	klog.Infof("trying to resolve %q using GCEResolver", addr)

	var seeds []string
	// TODO: Does it suffice to just query one zone (as long as we sort so it is always the first)?
	// Or does that introduce edges cases where we have partitions / cliques

	for _, zoneName := range r.zoneNames {
		pageToken := ""
		page := 0
		for ; page == 0 || (pageToken != "" && page < maxPages); page++ {
			listCall := r.compute.Instances.List(r.projectID, zoneName)

			// TODO: Filter by fields (but ask about google issue 29524655)

			// TODO: Match clusterid?

			if pageToken != "" {
				listCall.PageToken(pageToken)
			}

			res, err := listCall.Do()
			if err != nil {
				return nil, err
			}
			pageToken = res.NextPageToken
			for _, i := range res.Items {
				// TODO: Expose multiple IPs topologies?

				for _, ni := range i.NetworkInterfaces {
					// TODO: Check e.g. Network

					if ni.NetworkIP != "" {
						seeds = append(seeds, ni.NetworkIP)
					}
				}
			}
		}
		if page >= maxPages {
			klog.Errorf("GetSeeds exceeded maxPages=%d for Instances.List: truncating.", maxPages)
		}
	}

	klog.Infof("resolved %q => %v using GCEResolver", addr, seeds)

	return seeds, nil
}

func New() (*GCEResolver, error) {
	ctx := context.Background()

	computeService, err := compute.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("error building compute API client: %v", err)
	}

	zone, err := metadata.Zone()
	if err != nil {
		return nil, fmt.Errorf("failed to get zone from metadata: %w", err)
	}
	tokens := strings.Split(zone, "-")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("zone %q did not have expected format - cannot determine region", zone)
	}
	region := tokens[0] + "-" + tokens[1]

	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project id from metadata: %w", err)
	}

	zones, err := computeService.Zones.List(projectID).Do()
	if err != nil {
		return nil, fmt.Errorf("error querying for GCE zones: %w", err)
	}

	var zoneNames []string
	for _, zone := range zones.Items {
		regionName := lastComponent(zone.Region)
		if regionName != region {
			continue
		}
		zoneNames = append(zoneNames, zone.Name)
	}

	return &GCEResolver{
		compute:   computeService,
		zoneNames: zoneNames,
		projectID: projectID,
	}, nil
}

// Returns the last component of a URL, i.e. anything after the last slash
// If there is no slash, returns the whole string
func lastComponent(s string) string {
	lastSlash := strings.LastIndex(s, "/")
	if lastSlash != -1 {
		s = s[lastSlash+1:]
	}
	return s
}
