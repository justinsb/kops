/*
Copyright 2019 The Kubernetes Authors.

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

package config

import (
	nodeidentitygce "k8s.io/kops/pkg/nodeidentity/gce"
	"k8s.io/kops/upup/pkg/fi/cloudup/awsup"
)

type Options struct {
	Cloud      string         `json:"cloud,omitempty"`
	ConfigBase string         `json:"configBase,omitempty"`
	Server     *ServerOptions `json:"server,omitempty"`

	// ClusterAPI configures the cluster-api controllers.
	ClusterAPI *ClusterAPIOptions `json:"clusterAPI,omitempty"`

	// NodeController configures the node controllers.
	NodeController *NodeControllerOptions `json:"nodeController,omitempty"`

	CacheNodeidentityInfo bool `json:"cacheNodeidentityInfo,omitempty"`

	// LeaderElection configures leaderElection options.
	LeaderElection *LeaderElectionOptions `json:"leaderElection,omitempty"`
}

type LeaderElectionOptions struct {
	// Enabled controls whether we should use leader election to prevent multiple instances running.
	Enabled *bool `json:"enabled,omitempty"`
}

type NodeControllerOptions struct {
	// Enabled controls whether node controllers are enabled
	Enabled *bool `json:"enabled,omitempty"`
}

func (o *Options) PopulateDefaults() {
}

type ServerOptions struct {
	// Listen is the network endpoint (ip and port) we should listen on.
	Listen string

	// Provider is the cloud provider.
	Provider ServerProviderOptions `json:"provider"`

	// ServerKeyPath is the path to our TLS serving private key.
	ServerKeyPath string `json:"serverKeyPath,omitempty"`
	// ServerCertificatePath is the path to our TLS serving certificate.
	ServerCertificatePath string `json:"serverCertificatePath,omitempty"`

	// CABasePath is a base of the path to CA certificate and key files.
	CABasePath string `json:"caBasePath"`
	// SigningCAs is the list of active signing CAs.
	SigningCAs []string `json:"signingCAs"`
	// CertNames is the list of active certificate names.
	CertNames []string `json:"certNames"`
}

type ServerProviderOptions struct {
	AWS *awsup.AWSVerifierOptions        `json:"aws,omitempty"`
	GCE *nodeidentitygce.VerifierOptions `json:"gce,omitempty"`
}

type ClusterAPIOptions struct {
	// Enabled controls whether cluster api controllers are enabled
	Enabled *bool `json:"enabled,omitempty"`
}
