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
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/kops/dns-controller/pkg/dns"
	"k8s.io/kops/protokube/pkg/protokube"
	"k8s.io/kubernetes/federation/pkg/dnsprovider"
	"net"
	"os"
	"strings"

	// Load DNS plugins
	"k8s.io/kops/protokube/pkg/protokube/baremetal"
	_ "k8s.io/kubernetes/federation/pkg/dnsprovider/providers/aws/route53"
	_ "k8s.io/kubernetes/federation/pkg/dnsprovider/providers/google/clouddns"
)

var (
	flags = pflag.NewFlagSet("", pflag.ExitOnError)

	// value overwritten during build. This can be used to resolve issues.
	BuildVersion = "0.1"
)

func main() {
	fmt.Printf("protokube version %s\n", BuildVersion)

	err := run()
	if err != nil {
		glog.Errorf("Error: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	dnsProviderId := "aws-route53"
	flags.StringVar(&dnsProviderId, "dns", dnsProviderId, "DNS provider we should use (aws-route53, google-clouddns)")

	var zones []string
	flags.StringSliceVarP(&zones, "zone", "z", []string{}, "Configure permitted zones and their mappings")

	master := false
	flag.BoolVar(&master, "master", master, "Act as master")

	applyTaints := false
	flag.BoolVar(&applyTaints, "apply-taints", applyTaints, "Apply taints to nodes based on the role")

	containerized := false
	flag.BoolVar(&containerized, "containerized", containerized, "Set if we are running containerized.")

	cloud := "aws"
	flag.StringVar(&cloud, "cloud", cloud, "CloudProvider we are using (aws,gce,baremetal)")

	populateExternalIP := false
	flag.BoolVar(&populateExternalIP, "populate-external-ip", populateExternalIP, "If set, will populate the external IP when starting up")

	dnsInternalSuffix := ""
	flag.StringVar(&dnsInternalSuffix, "dns-internal-suffix", dnsInternalSuffix, "DNS suffix for internal domain names")

	clusterID := ""
	flag.StringVar(&clusterID, "cluster-id", clusterID, "Cluster ID")

	flagChannels := ""
	flag.StringVar(&flagChannels, "channels", flagChannels, "channels to install")

	// Trick to avoid 'logging before flag.Parse' warning
	flag.CommandLine.Parse([]string{})

	flag.Set("logtostderr", "true")

	flags.AddGoFlagSet(flag.CommandLine)

	flags.Parse(os.Args)

	rootfs := "/"
	if containerized {
		rootfs = "/rootfs/"
	}
	protokube.RootFS = rootfs

	var volumes protokube.Volumes
	var internalIP net.IP
	var externalIPs []net.IP

	if cloud == "aws" {
		awsVolumes, err := protokube.NewAWSVolumes()
		if err != nil {
			glog.Errorf("Error initializing AWS: %q", err)
			os.Exit(1)
		}
		volumes = awsVolumes

		if clusterID == "" {
			clusterID = awsVolumes.ClusterID()
		}
		if internalIP == nil {
			internalIP = awsVolumes.InternalIP()
		}
	} else if cloud == "gce" {
		gceVolumes, err := protokube.NewGCEVolumes()
		if err != nil {
			glog.Errorf("Error initializing GCE: %q", err)
			os.Exit(1)
		}

		volumes = gceVolumes

		//gceProject = gceVolumes.Project()

		if clusterID == "" {
			clusterID = gceVolumes.ClusterID()
		}

		if internalIP == nil {
			internalIP = gceVolumes.InternalIP()
		}
	} else if cloud == "baremetal" {
		basedir := protokube.PathFor("/volumes")
		baremetalVolumes, err := baremetal.NewVolumes(basedir)
		if err != nil {
			glog.Errorf("Error initializing baremetal: %q", err)
			os.Exit(1)
		}

		volumes = baremetalVolumes

		externalIPs, err = protokube.FindExternalIPs()
		if err != nil {
			glog.Errorf("Error finding external IP: %q", err)
			os.Exit(1)
		} else {
			glog.Infof("Found external IPs %s", externalIPs)
		}
	} else {
		glog.Errorf("Unknown cloud %q", cloud)
		os.Exit(1)
	}

	internalIP, err := protokube.FindInternalIP()
	if err != nil {
		glog.Errorf("Error finding internal IP: %q", err)
		os.Exit(1)
	}

	if internalIP == nil {
		glog.Errorf("Cannot determine internal IP")
		os.Exit(1)
	}

	if populateExternalIP && len(externalIPs) == 0 {
		glog.Errorf("Cannot determine external IPs")
		os.Exit(1)
	}

	if dnsInternalSuffix == "" {
		if clusterID == "" {
			if clusterID == "" {
				return fmt.Errorf("cluster-id is required (cannot be determined from cloud)")
			} else {
				glog.Infof("Setting cluster-id from cloud: %s", clusterID)
			}
		}

		// TODO: Maybe only master needs DNS?
		dnsInternalSuffix = ".internal." + clusterID
		glog.Infof("Setting dns-internal-suffix to %q", dnsInternalSuffix)
	}

	// Make sure it's actually a suffix (starts with .)
	if !strings.HasPrefix(dnsInternalSuffix, ".") {
		dnsInternalSuffix = "." + dnsInternalSuffix
	}

	// Get internal IP from cloud, to avoid problems if we're in a container
	// TODO: Just run with --net=host ??
	//internalIP, err := findInternalIP()
	//if err != nil {
	//	glog.Errorf("Error finding internal IP: %q", err)
	//	os.Exit(1)
	//}

	var dnsScope dns.Scope
	var dnsController *dns.DNSController
	{
		dnsProvider, err := dnsprovider.GetDnsProvider(dnsProviderId, nil)
		if err != nil {
			return fmt.Errorf("Error initializing DNS provider %q: %v", dnsProviderId, err)
		}
		if dnsProvider == nil {
			return fmt.Errorf("DNS provider %q could not be initialized", dnsProviderId)
		}

		zoneRules, err := dns.ParseZoneRules(zones)
		if err != nil {
			return fmt.Errorf("unexpected zone flags: %q", err)
		}

		dnsController, err = dns.NewDNSController(dnsProvider, zoneRules)
		if err != nil {
			return err
		}

		dnsScope, err = dnsController.CreateScope("protokube")
		if err != nil {
			return err
		}

		// We don't really use readiness - our records are simple
		dnsScope.MarkReady()
	}

	protokube.Containerized = containerized

	modelDir := "model/etcd"

	var channels []string
	if flagChannels != "" {
		channels = strings.Split(flagChannels, ",")
	}

	k := &protokube.KubeBoot{
		Master:            master,
		ApplyTaints:       applyTaints,
		InternalDNSSuffix: dnsInternalSuffix,
		InternalIP:        internalIP,
		ExternalIPs:       externalIPs,
		//MasterID          : fromVolume
		//EtcdClusters   : fromVolume

		ModelDir: modelDir,
		DNSScope: dnsScope,

		Channels: channels,

		Kubernetes: protokube.NewKubernetesContext(),

		PopulateExternalIP: populateExternalIP,
	}
	k.Init(volumes)

	go dnsController.Run()

	k.RunSyncLoop()

	return fmt.Errorf("Unexpected exit")
}
