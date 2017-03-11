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
	"github.com/golang/glog"
	"k8s.io/kops/tools/hostdisco/pkg/seeds/static"
	"k8s.io/kops/tools/hostdisco/pkg/seeds"
	"fmt"
	"k8s.io/kops/tools/hostdisco/pkg/gossip"
)

func main() {
	glog.Infof("Hello world")

	seedDiscovery := static.New([]string{"127.0.0.1" })

	err := run(seedDiscovery)
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}
}

func run(seedDiscovery seeds.SeedDiscovery) error {
	listenAddress := ":9999"
	server := gossip.NewServer(listenAddress)
	err := server.Run()
	if err !=nil {
		return fmt.Errorf("error running gossip server: %v", err)
	}

	seeds, err := seedDiscovery.GetSeeds()
	if err != nil {
		// TODO: Retry loop
		return fmt.Errorf("error discovery seeds: %v", err)
	}

	for _, seed := range seeds {
		glog.Infof("found seed: %v", seed)
	}

	return nil
}
