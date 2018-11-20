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

	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/pools"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/openstack"
)

//go:generate fitask -type=LBPool
type LBPool struct {
	ID           *string
	Name         *string
	Lifecycle    *fi.Lifecycle
	Loadbalancer *LB
}

// GetDependencies returns the dependencies of the Instance task
func (e *LBPool) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	var deps []fi.Task
	for _, task := range tasks {
		if _, ok := task.(*LB); ok {
			deps = append(deps, task)
		}
	}
	return deps
}

var _ fi.CompareWithID = &LBPool{}

func (s *LBPool) CompareWithID() *string {
	return s.ID
}

func NewLBPoolTaskFromCloud(cloud openstack.OpenstackCloud, lifecycle *fi.Lifecycle, pool *pools.Pool) (*LBPool, error) {

	if len(pool.Loadbalancers) > 1 {
		return nil, fmt.Errorf("Openstack cloud pools with multiple loadbalancers not yet supported!")
	}

	a := &LBPool{
		ID:   fi.String(pool.ID),
		Name: fi.String(pool.Name),
	}
	if len(pool.Loadbalancers) == 1 {
		lbID := pool.Loadbalancers[0]
		lb, err := cloud.GetLB(lbID.ID)
		if err != nil {
			return nil, fmt.Errorf("NewLBPoolTaskFromCloud: Failed to get lb with id %s: %v", lbID.ID, err)
		}
		loadbalancerTask, err := NewLBTaskFromCloud(cloud, lifecycle, lb)
		if err != nil {
			return nil, err
		}
		a.Loadbalancer = loadbalancerTask
	}
	return a, nil
}

func (p *LBPool) Find(context *fi.Context) (*LBPool, error) {
	if p.ID == nil {
		return nil, nil
	}

	cloud := context.Cloud.(openstack.OpenstackCloud)
	// TODO: Move to cloud
	pool, err := pools.Get(cloud.LoadBalancerClient(), fi.StringValue(p.ID)).Extract()
	if err != nil {
		return nil, err
	}

	return NewLBPoolTaskFromCloud(cloud, p.Lifecycle, pool)
}

func (s *LBPool) Run(context *fi.Context) error {
	return fi.DefaultDeltaRunMethod(s, context)
}

func (_ *LBPool) CheckChanges(a, e, changes *LBPool) error {
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

func (_ *LBPool) RenderOpenstack(t *openstack.OpenstackAPITarget, a, e, changes *LBPool) error {
	if a == nil {

		poolopts := pools.CreateOpts{
			Name:           fi.StringValue(e.Name),
			LBMethod:       pools.LBMethodRoundRobin,
			Protocol:       pools.ProtocolTCP,
			LoadbalancerID: fi.StringValue(e.Loadbalancer.ID),
		}
		pool, err := pools.Create(t.Cloud.LoadBalancerClient(), poolopts).Extract()
		if err != nil {
			return fmt.Errorf("error creating LB pool: %v", err)
		}
		e.ID = fi.String(pool.ID)

		return nil
	}

	glog.V(2).Infof("Openstack task LB::RenderOpenstack did nothing")
	return nil
}
