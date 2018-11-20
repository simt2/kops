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

package openstacktasks

import (
	"fmt"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"

	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/pools"
	v2pools "github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/lbaas_v2/pools"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/openstack"
)

//go:generate fitask -type=PoolAssociation
type PoolAssociation struct {
	ID            *string
	Name          *string
	Lifecycle     *fi.Lifecycle
	Pool          *LBPool
	ServerGroup   *ServerGroup
	InterfaceName *string
	ProtocolPort  *int
}

// GetDependencies returns the dependencies of the Instance task
func (e *PoolAssociation) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	var deps []fi.Task
	for _, task := range tasks {
		if _, ok := task.(*LB); ok {
			deps = append(deps, task)
		}
		if _, ok := task.(*LBPool); ok {
			deps = append(deps, task)
		}
		if _, ok := task.(*Instance); ok {
			deps = append(deps, task)
		}
	}
	return deps
}

var _ fi.CompareWithID = &PoolAssociation{}

func (s *PoolAssociation) CompareWithID() *string {
	return s.ID
}

func (p *PoolAssociation) Find(context *fi.Context) (*PoolAssociation, error) {
	if p.ID == nil {
		return nil, nil
	}

	cloud := context.Cloud.(openstack.OpenstackCloud)
	// TODO: Move to cloud
	_, err := pools.Get(cloud.LoadBalancerClient(), fi.StringValue(p.ID)).Extract()
	if err != nil {
		return nil, err
	}

	// TODO:
	return nil, nil
}

func (s *PoolAssociation) Run(context *fi.Context) error {
	return fi.DefaultDeltaRunMethod(s, context)
}

func (_ *PoolAssociation) CheckChanges(a, e, changes *PoolAssociation) error {
	if a == nil {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
	} else {
		if changes.ID != nil {
			return fi.CannotChangeField("ID")
		}
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
	}
	return nil
}

func (_ *PoolAssociation) RenderOpenstack(t *openstack.OpenstackAPITarget, a, e, changes *PoolAssociation) error {
	if a == nil {

		for _, serverID := range e.ServerGroup.Members {
			server, err := servers.Get(t.Cloud.ComputeClient(), serverID).Extract()
			if err != nil {
				return fmt.Errorf("Failed to find server with id `%s`: %v", serverID, err)
			}

			var poolAddress string
			if localAddr, ok := server.Addresses[fi.StringValue(e.InterfaceName)]; ok {

				if localAddresses, ok := localAddr.([]interface{}); ok {
					for _, addr := range localAddresses {
						addrMap := addr.(map[string]interface{})
						if addrType, ok := addrMap["OS-EXT-IPS:type"]; ok && addrType == "fixed" {
							if fixedIP, ok := addrMap["addr"]; ok {
								if fixedIPStr, ok := fixedIP.(string); ok {
									poolAddress = fixedIPStr
								} else {
									err = fmt.Errorf("Fixed IP was not a string: %v", fixedIP)
								}
							} else {
								err = fmt.Errorf("Type fixed did not contain addr: %v", addr)
							}
						}
					}
				}
			} else {
				err = fmt.Errorf("Server `%s` interface name `%s` not found!", serverID, fi.StringValue(e.InterfaceName))
			}

			if err != nil {
				return fmt.Errorf("Failed to get fixed ip for associated pool: %v", err)
			}

			association, err := v2pools.CreateMember(t.Cloud.NetworkingClient(), fi.StringValue(e.Pool.ID), v2pools.CreateMemberOpts{
				Name:         fi.StringValue(e.Name),
				ProtocolPort: fi.IntValue(e.ProtocolPort),
				SubnetID:     fi.StringValue(e.Pool.Loadbalancer.VipSubnet),
				Address:      poolAddress,
			}).Extract()
			if err != nil {
				return fmt.Errorf("Failed to address %s to pool %s: %v", poolAddress, fi.StringValue(e.Name), err)
			}
			e.ID = fi.String(association.ID)
		}

		return nil
	}

	glog.V(2).Infof("Openstack task LB::RenderOpenstack did nothing")
	return nil
}
