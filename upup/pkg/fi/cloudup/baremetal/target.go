package baremetal

import (
	"k8s.io/kops/upup/pkg/fi"
)

type Target struct {
	cloud *Cloud
}

var _ fi.Target = &Target{}

func NewTarget(cloud *Cloud) *Target {
	return &Target{cloud: cloud}
}

func (t *Target) Finish(taskMap map[string]fi.Task) error {
	return nil
}

// ProcessDeletions returns true if we should delete resources
// Some providers (e.g. Terraform) actively keep state, and will delete resources automatically
func (t *Target) ProcessDeletions() bool {
	return true
}
