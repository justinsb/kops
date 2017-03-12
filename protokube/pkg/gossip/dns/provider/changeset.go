package provider

import (
	"k8s.io/kubernetes/federation/pkg/dnsprovider"
)

type resourceRecordChangeset struct {
	zone   *zone
	add    []dnsprovider.ResourceRecordSet
	remove []dnsprovider.ResourceRecordSet
}

var _ dnsprovider.ResourceRecordChangeset = &resourceRecordChangeset{}

// Add adds the creation of a ResourceRecordSet in the Zone to the changeset
func (c *resourceRecordChangeset) Add(rrs dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordChangeset {
	c.add = append(c.add, rrs)
	return c
}

// Remove adds the removal of a ResourceRecordSet in the Zone to the changeset
// The supplied ResourceRecordSet must match one of the existing recordsets (obtained via List()) exactly.
func (c *resourceRecordChangeset) Remove(rrs dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordChangeset {
	c.remove = append(c.remove, rrs)
	return c
}

// Apply applies the accumulated operations to the Zone.
func (c *resourceRecordChangeset) Apply() error {
	return c.zone.applyChangeset(c)
}
