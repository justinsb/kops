/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"github.com/blang/semver"
	"io/ioutil"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"path"
	"testing"
)

// TestKopsUpgrades tests the version logic for kops versions
func TestKopsUpgrades(t *testing.T) {
	srcDir := "simple"
	sourcePath := path.Join(srcDir, "channel.yaml")
	sourceBytes, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error reading sourcePath %q: %v", sourcePath, err)
	}

	channel, err := kops.ParseChannel(sourceBytes)
	if err != nil {
		t.Fatalf("failed to parse channel: %v", err)
	}

	grid := []struct {
		KopsVersion      string
		ExpectedUpgrade  string
		ExpectedRequired bool
		ExpectedError    bool
	}{
		{
			KopsVersion:      "1.4.4",
			ExpectedUpgrade:  "1.4.5",
			ExpectedRequired: true,
		},
		{
			KopsVersion:     "1.4.5",
			ExpectedUpgrade: "",
		},
		{
			KopsVersion:      "1.5.0-alpha4",
			ExpectedUpgrade:  "1.5.0-beta1",
			ExpectedRequired: true,
		},
		{
			KopsVersion:     "1.5.0-beta1",
			ExpectedUpgrade: "",
		},
		{
			KopsVersion:     "1.5.0-beta2",
			ExpectedUpgrade: "",
		},
		{
			KopsVersion:      "1.5.0",
			ExpectedUpgrade:  "1.5.1",
			ExpectedRequired: false,
		},
		{
			KopsVersion:     "1.5.1",
			ExpectedUpgrade: "",
		},
	}
	for _, g := range grid {
		kopsVersion := semver.MustParse(g.KopsVersion)

		versionInfo := kops.FindKopsVersionSpec(channel.Spec.KopsVersions, kopsVersion)
		if versionInfo == nil {
			t.Errorf("unable to find version information for kops version %q in channel", kopsVersion)
			continue
		}

		actual, err := versionInfo.FindRecommendedUpgrade(kopsVersion)
		if g.ExpectedError {
			if err == nil {
				t.Errorf("expected error from FindRecommendedUpgrade(%q)", g.KopsVersion)
				continue
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error from FindRecommendedUpgrade(%q): %v", g.KopsVersion, err)
				continue
			}
		}
		if semverString(actual) != g.ExpectedUpgrade {
			t.Errorf("unexpected result from IsUpgradeRequired(%q): expected=%q, actual=%q", g.KopsVersion, g.ExpectedUpgrade, actual)
			continue
		}

		required, err := versionInfo.IsUpgradeRequired(kopsVersion)
		if err != nil {
			t.Errorf("unexpected error from IsUpgradeRequired(%q)", g.KopsVersion, err)
			continue
		}
		if required != g.ExpectedRequired {
			t.Errorf("unexpected result from IsUpgradeRequired(%q): expected=%t, actual=%t", g.KopsVersion, g.ExpectedRequired, required)
			continue
		}
	}
}

// TestKubernetesUpgrades tests the version logic kubernetes kops versions
func TestKubernetesUpgrades(t *testing.T) {
	srcDir := "simple"
	sourcePath := path.Join(srcDir, "channel.yaml")
	sourceBytes, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error reading sourcePath %q: %v", sourcePath, err)
	}

	channel, err := kops.ParseChannel(sourceBytes)
	if err != nil {
		t.Fatalf("failed to parse channel: %v", err)
	}

	grid := []struct {
		KubernetesVersion string
		ExpectedUpgrade   string
		ExpectedRequired  bool
		ExpectedError     bool
	}{
		{
			KubernetesVersion: "1.4.0",
			ExpectedUpgrade:   "1.4.8",
			ExpectedRequired:  true,
		},
		{
			KubernetesVersion: "1.4.1",
			ExpectedUpgrade:   "1.4.8",
			ExpectedRequired:  true,
		},
		{
			KubernetesVersion: "1.4.2",
			ExpectedUpgrade:   "1.4.8",
		},
		{
			KubernetesVersion: "1.4.4",
			ExpectedUpgrade:   "1.4.8",
		},
		{
			KubernetesVersion: "1.4.8",
			ExpectedUpgrade:   "",
		},
		{
			KubernetesVersion: "1.5.0",
			ExpectedUpgrade:   "1.5.2",
			ExpectedRequired:  true,
		},
		{
			KubernetesVersion: "1.5.1",
			ExpectedUpgrade:   "1.5.2",
		},
		{
			KubernetesVersion: "1.5.2",
			ExpectedUpgrade:   "",
		},
	}
	for _, g := range grid {
		kubernetesVersion := semver.MustParse(g.KubernetesVersion)

		versionInfo := kops.FindKubernetesVersionSpec(channel.Spec.KubernetesVersions, kubernetesVersion)
		if versionInfo == nil {
			t.Errorf("unable to find version information for kubernetes version %q in channel", kubernetesVersion)
			continue
		}

		actual, err := versionInfo.FindRecommendedUpgrade(kubernetesVersion)
		if g.ExpectedError {
			if err == nil {
				t.Errorf("expected error from FindRecommendedUpgrade(%q)", g.KubernetesVersion)
				continue
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error from FindRecommendedUpgrade(%q): %v", g.KubernetesVersion, err)
				continue
			}
		}
		if semverString(actual) != g.ExpectedUpgrade {
			t.Errorf("unexpected result from IsUpgradeRequired(%q): expected=%q, actual=%q", g.KubernetesVersion, g.ExpectedUpgrade, actual)
			continue
		}

		required, err := versionInfo.IsUpgradeRequired(kubernetesVersion)
		if err != nil {
			t.Errorf("unexpected error from IsUpgradeRequired(%q)", g.KubernetesVersion, err)
			continue
		}
		if required != g.ExpectedRequired {
			t.Errorf("unexpected result from IsUpgradeRequired(%q): expected=%t, actual=%t", g.KubernetesVersion, g.ExpectedRequired, required)
			continue
		}
	}
}

// TestFindImage tests the version-based image finding
func TestFindImage(t *testing.T) {
	srcDir := "simple"
	sourcePath := path.Join(srcDir, "channel.yaml")
	sourceBytes, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error reading sourcePath %q: %v", sourcePath, err)
	}

	channel, err := kops.ParseChannel(sourceBytes)
	if err != nil {
		t.Fatalf("failed to parse channel: %v", err)
	}

	grid := []struct {
		KubernetesVersion string
		ExpectedImage     string
	}{
		{
			KubernetesVersion: "1.4.4",
			ExpectedImage:     "kope.io/k8s-1.4-debian-jessie-amd64-hvm-ebs-2016-10-21",
		},
		{
			KubernetesVersion: "1.5.1",
			ExpectedImage:     "kope.io/k8s-1.5-debian-jessie-amd64-hvm-ebs-2017-01-09",
		},
	}
	for _, g := range grid {
		kubernetesVersion := semver.MustParse(g.KubernetesVersion)

		image := channel.FindImage(fi.CloudProviderAWS, kubernetesVersion)
		name := ""
		if image != nil {
			name = image.Name
		}
		if name != g.ExpectedImage {
			t.Errorf("unexpected image from FindImage(%q): expected=%q, actual=%q", g.KubernetesVersion, g.ExpectedImage, name)
		}
	}
}

// TestRecommendedKubernetesVersion tests the version logic kubernetes kops versions
func TestRecommendedKubernetesVersion(t *testing.T) {
	srcDir := "simple"
	sourcePath := path.Join(srcDir, "channel.yaml")
	sourceBytes, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("unexpected error reading sourcePath %q: %v", sourcePath, err)
	}

	channel, err := kops.ParseChannel(sourceBytes)
	if err != nil {
		t.Fatalf("failed to parse channel: %v", err)
	}

	grid := []struct {
		KopsVersion               string
		ExpectedKubernetesVersion string
	}{
		{
			KopsVersion:               "1.4.4",
			ExpectedKubernetesVersion: "1.4.8",
		},
		{
			KopsVersion:               "1.4.5",
			ExpectedKubernetesVersion: "1.4.8",
		},
		{
			KopsVersion:               "1.5.0",
			ExpectedKubernetesVersion: "1.5.2",
		},
		{
			KopsVersion:               "1.5.0-beta2",
			ExpectedKubernetesVersion: "1.5.0",
		},
	}
	for _, g := range grid {
		kubernetesVersion := kops.RecommendedKubernetesVersion(channel, g.KopsVersion)
		if semverString(kubernetesVersion) != g.ExpectedKubernetesVersion {
			t.Errorf("unexpected result from RecommendedKubernetesVersion(%q): expected=%q, actual=%q", g.KopsVersion, g.ExpectedKubernetesVersion, semverString(kubernetesVersion))
			continue
		}
	}
}

func TestOrdering(t *testing.T) {
	if !semver.MustParse("1.5.0").GTE(semver.MustParse("1.5.0-alpha1")) {
		t.Fatalf("Expected: 1.5.0 >= 1.5.0-alpha1")
	}

	if !semver.MustParseRange(">=1.5.0-alpha1")(semver.MustParse("1.5.0")) {
		t.Fatalf("Expected: '>=1.5.0-alpha1' to include 1.5.0")
	}
}

func semverString(sv *semver.Version) string {
	if sv == nil {
		return ""
	}
	return sv.String()
}
