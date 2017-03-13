package validation

import (
	"k8s.io/kops/pkg/apis/kops"
	"testing"
)

func TestValidateInstanceGroupSpec(t *testing.T) {
	grid := []struct {
		Input          kops.InstanceGroupSpec
		ExpectedErrors []string
	}{
		{
			Input: kops.InstanceGroupSpec{
				AdditionalSecurityGroups: []string{},
			},
		},
		{
			Input: kops.InstanceGroupSpec{
				AdditionalSecurityGroups: []string{"sg-1234abcd"},
			},
		},
		{
			Input: kops.InstanceGroupSpec{
				AdditionalSecurityGroups: []string{"sg-1234abcd", ""},
			},
			ExpectedErrors: []string{"Invalid value::spec.additionalSecurityGroups[1]"},
		},
		{
			Input: kops.InstanceGroupSpec{
				AdditionalSecurityGroups: []string{" ", ""},
			},
			ExpectedErrors: []string{
				"Invalid value::spec.additionalSecurityGroups[0]",
				"Invalid value::spec.additionalSecurityGroups[1]",
			},
		},
		{
			Input: kops.InstanceGroupSpec{
				AdditionalSecurityGroups: []string{"--invalid"},
			},
			ExpectedErrors: []string{"Invalid value::spec.additionalSecurityGroups[0]"},
		},
	}
	for _, g := range grid {
		ig := &kops.InstanceGroup{
			Spec: g.Input,
		}
		errs := awsValidateInstanceGroup(ig)

		testErrors(t, g.Input, errs, g.ExpectedErrors)
	}
}
