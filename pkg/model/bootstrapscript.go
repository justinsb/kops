package model

import (
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/nodeup"
	"k8s.io/kops/pkg/model/resources"
	"k8s.io/kops/upup/pkg/fi"
	"text/template"
)

// BootstrapScript creates the bootstrap script
type BootstrapScript struct {
	NodeUpSource        string
	NodeUpSourceHash    string
	NodeUpConfigBuilder func(ig *kops.InstanceGroup) (*nodeup.NodeUpConfig, error)
}

func (b *BootstrapScript) ResourceNodeUp(ig *kops.InstanceGroup) (*fi.ResourceHolder, error) {
	if ig.Spec.Role == kops.InstanceGroupRoleBastion {
		// Bastions are just bare machines (currently), used as SSH jump-hosts
		return nil, nil
	}

	functions := template.FuncMap{
		"NodeUpSource": func() string {
			return b.NodeUpSource
		},
		"NodeUpSourceHash": func() string {
			return b.NodeUpSourceHash
		},
		"KubeEnv": func() (string, error) {
			config, err := b.NodeUpConfigBuilder(ig)
			if err != nil {
				return "", err
			}

			data, err := kops.ToRawYaml(config)
			if err != nil {
				return "", err
			}

			return string(data), nil
		},
	}

	templateResource, err := NewTemplateResource("nodeup", resources.AWSNodeUpTemplate, functions, nil)
	if err != nil {
		return nil, err
	}
	return fi.WrapResource(templateResource), nil
}
