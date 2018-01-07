/*
Copyright 2017 The Kubernetes Authors.

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
	"github.com/spf13/pflag"
)

type Configuration struct {
	Kubeconfig        string
	Cloud             string
	SshUsername       string
	SshPrivateKeyPath string
	ConfigBase        string
	ClusterName       string
	ControllerId      string
}

func NewConfiguration() *Configuration {
	c := &Configuration{
		SshUsername:       "root",
		SshPrivateKeyPath: "/etc/kubernetes/kops/ssh/id_rsa",
	}
	return c
}

func (c *Configuration) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Kubeconfig, "kubeconfig", c.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.StringVar(&c.Cloud, "cloud", c.Cloud, "Cloud provider (google/azure).")
	fs.StringVar(&c.SshUsername, "ssh-username", c.SshUsername, "Username to use for SSH communications with machines.")
	fs.StringVar(&c.SshPrivateKeyPath, "ssh-private-key", c.SshPrivateKeyPath, "Path to SSH private key to use for SSH communications with machines.")
	fs.StringVar(&c.ConfigBase, "config", c.ConfigBase, "VFS path where cluster configuration is stored.")
	fs.StringVar(&c.ClusterName, "cluster", c.ClusterName, "Name of cluster in config store.")
	fs.StringVar(&c.ControllerId, "controller-id", c.ControllerId, "Identifier for controller; if set we will only create instances with the matching controller id")
}
