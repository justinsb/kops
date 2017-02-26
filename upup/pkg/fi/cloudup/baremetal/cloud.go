package baremetal

import (
	"fmt"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kubernetes/federation/pkg/dnsprovider"
)

type Cloud struct {
	dns dnsprovider.Interface
}

var _ fi.Cloud = &Cloud{}

func NewCloud(dns dnsprovider.Interface) (*Cloud, error) {
	return &Cloud{dns: dns}, nil
}

func (c *Cloud) ProviderID() fi.CloudProviderID {
	return fi.CloudProviderBareMetal
}

func (c *Cloud) DNS() (dnsprovider.Interface, error) {
	return c.dns, nil
}

func (c *Cloud) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	return nil, fmt.Errorf("No network information on baremetal")
}
