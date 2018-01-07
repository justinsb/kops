package vsphere

import (
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/find"
	"fmt"
	"context"
	"github.com/golang/glog"
)

type VsphereContext struct {
	client       *vim25.Client
	datastore    *object.Datastore
	datacenter   *object.Datacenter
	resourcePool *object.ResourcePool
	finder       *find.Finder
}

func NewVsphereContext(client *vim25.Client) (*VsphereContext, error) {
	f := find.NewFinder(client, true)

	ctx := context.TODO()

	//es, err := f.ManagedObjectListChildren(ctx, "/ha-datacenter")
	//if err != nil {
	//	return err
	//}
	//for _, e := range es {
	//	glog.Infof("Found child %q", e.Path)
	//}
	datacenter, err := f.DatacenterOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error getting datacenter: %v", err)
	}
	glog.Infof("using datacenter %q", datacenter.Name())
	f.SetDatacenter(datacenter)

	datastore, err := f.DatastoreOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error finding datastore: %v", err)
	}
	glog.Infof("using datastore %q", datastore.Name())

	resourcePool, err := f.ResourcePoolOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error getting resource pool: %v", err)
	}
	glog.Infof("using resource pool %q", resourcePool.Name())

	return &VsphereContext{
		client:       client,
		datastore:    datastore,
		datacenter:   datacenter,
		finder:       f,
		resourcePool: resourcePool,
	}, nil
}
