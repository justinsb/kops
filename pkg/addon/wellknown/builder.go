package wellknown

import (
	"k8s.io/api/core/v1"
	"k8s.io/kops/pkg/addon"
	"k8s.io/kops/pkg/apis/kops"
)

func BuildAddons(c *kops.Cluster) ([]*v1.ConfigMap, error) {
	var addons []*v1.ConfigMap

	kubeDNS := c.Spec.KubeDNS
	if kubeDNS == nil {
		kubeDNS = &kops.KubeDNSConfig{}
	}
	if kubeDNS.Provider == "CoreDNS" {
		channelURL := "https://raw.githubusercontent.com/justinsb/kops/managedaddons/addons/coredns.io/coredns/stable"
		bundle, err := addon.Load(channelURL)
		if err != nil {
			return nil, err
		}
		cm := bundle.Persist()
		cm.Namespace = "kube-system"
		cm.Name = "coredns"
		addons = append(addons, cm)
	}

	return addons, nil
}
