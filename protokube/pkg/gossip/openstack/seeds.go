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

package gce

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/pagination"
	"k8s.io/kops/protokube/pkg/gossip"
)

type SeedProvider struct {
	computeClient *gophercloud.ServiceClient
	projectID     string
	clusterName   string
}

var _ gossip.SeedProvider = &SeedProvider{}

func (p *SeedProvider) GetSeeds() ([]string, error) {
	var seeds []string

	err := servers.List(p.computeClient, servers.ListOpts{
		TenantID: p.projectID,
	}).EachPage(func(page pagination.Page) (bool, error) {
		var s []servers.Server
		err := servers.ExtractServersInto(page, &s)
		if err != nil {
			return false, err
		}

		for _, server := range s {
			if clusterName, ok := server.Metadata["cluster_name"]; ok {
				var err error
				if localAddr, ok := server.Addresses[clusterName]; ok {

					if localAddresses, ok := localAddr.([]interface{}); ok {
						for _, addr := range localAddresses {
							addrMap := addr.(map[string]interface{})
							if addrType, ok := addrMap["OS-EXT-IPS:type"]; ok && addrType == "fixed" {
								if fixedIP, ok := addrMap["addr"]; ok {
									if fixedIPStr, ok := fixedIP.(string); ok {
										seeds = append(seeds, fixedIPStr)
									} else {
										err = fmt.Errorf("Fixed IP was not a string: %v", fixedIP)
									}
								} else {
									err = fmt.Errorf("Type fixed did not contain addr: %v", addr)
								}
							} else {
								err = fmt.Errorf("Server %s did not contain a fixed IP.", server.ID)
							}
						}
					}
				} else {
					err = fmt.Errorf("Server `%s` interface name `%s` not found!", server.ID, clusterName)
				}
				if err != nil {
					glog.Warningf("Failed to list seeds: %v", err)
				}
			}
		}
		return true, nil
	})

	if err != nil {
		return seeds, fmt.Errorf("Failed to list servers while retrieving seeds: %v", err)
	}

	return seeds, nil
}

func NewSeedProvider(computeClient *gophercloud.ServiceClient, clusterName string, projectID string) (*SeedProvider, error) {
	return &SeedProvider{
		computeClient: computeClient,
		clusterName:   clusterName,
		projectID:     projectID,
	}, nil
}
