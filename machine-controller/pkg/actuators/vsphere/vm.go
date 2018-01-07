package vsphere

import (
	"fmt"
	"os"
	"github.com/golang/glog"
	"k8s.io/kops/upup/pkg/fi/cloudup/vsphere"
	"context"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/object"
	"io/ioutil"
	"path/filepath"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/ovf"
	"errors"
	"github.com/vmware/govmomi/vim25"
	"net/url"
	"github.com/ghodss/yaml"
)

func (a *VsphereActuator) createVMRaw(ctx context.Context, vsphereContext *VsphereContext, cloud *vsphere.VSphereCloud, name string) (*object.VirtualMachine, error) {

	//vms, err := f.VirtualMachineList(ctx, name)
	//if err != nil {
	//	return fmt.Errorf("error listing virtual machines: %v", err)
	//}
	//for _, vm := range vms {
	//	glog.Infof("Found vm %q", vm.Name())
	//	vmUuid, err := cloud.FindVMUUID(vm.Name())
	//	if err != nil {
	//		return fmt.Errorf("error getting uuid for vm %q: %v", vm.Name(), err)
	//	}
	//	glog.Infof("vm uuid is %q", vmUuid)
	//}

	hosts, err := vsphereContext.finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("error listing hosts: %v", err)
	}
	for _, host := range hosts {
		glog.Infof("Found host %q", host.Name())
	}

	computeResources, err := vsphereContext.finder.ComputeResourceList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("error listing computeResources: %v", err)
	}
	for _, computeResource := range computeResources {
		glog.Infof("Found ComputeResource %q", computeResource.Name())
	}

	network, err := vsphereContext.finder.NetworkOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error finding network: %v", err)
	}
	glog.Infof("using network %q", network.Reference())

	folder, err := vsphereContext.finder.FolderOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error finding folder: %v", err)
	}
	glog.Infof("using folder %q", folder.Reference())

	var cloudconfigData cloudconfigData
	cloudconfigData = a.cloudconfigData
	cloudconfigData.Hostname = name

	vmxIsoPath := fmt.Sprintf("vms/%s/cloud-init.iso", name)
	if err := a.uploadISO(ctx, vsphereContext, &cloudconfigData, vmxIsoPath); err != nil {
		return nil, err
	}

	{
		//http://pubs.vmware.com/vsphere-6-5/topic/com.vmware.wssdk.apiref.doc/vim.vm.GuestOsDescriptor.GuestOsIdentifier.html
		guestID := "coreos64Guest"
		cpus := 2
		memory := 4096
		description := ""
		//image := "xenial/xenial.vmdk"
		image := "core/core.vmdk"
		//image := "artful/artful.vmdk"
		linkImage := true
		adapter := "vmxnet3" // "e1000"
		diskDriver := "pvscsi"

		var devices object.VirtualDeviceList

		vmSpec := &types.VirtualMachineConfigSpec{
			Name:       name,
			GuestId:    guestID,
			NumCPUs:    int32(cpus),
			MemoryMB:   int64(memory),
			Annotation: description,
			//Firmware: "efi",
		}

		// Create the SCSI controller for mounting the disk
		var diskDevice types.BaseVirtualDevice
		{
			scsi, err := devices.CreateSCSIController(diskDriver)
			if err != nil {
				return nil, fmt.Errorf("error creating SCSI controller: %v", err)
			}
			devices = append(devices, scsi)
			diskDevice = scsi
		}

		// Create an IDE controller for mounting the ISO
		var isoDevice types.BaseVirtualDevice
		{
			ide, err := devices.CreateIDEController()
			if err != nil {
				return nil, fmt.Errorf("error creating ide controller: %v", err)
			}
			devices = append(devices, ide)
			isoDevice = ide
		}

		// Mount the disk
		{
			controller, err := devices.FindDiskController(devices.Name(diskDevice))
			if err != nil {
				return nil, fmt.Errorf("error finding disk controller: %v", err)
			}

			diskStat, err := vsphereContext.datastore.Stat(ctx, image)
			if err != nil {
				return nil, fmt.Errorf("error reading image %q: %v", image, err)
			}
			glog.Infof("using image %q: %v", image, diskStat)

			path := vsphereContext.datastore.Path(image)
			disk := devices.CreateDisk(controller, vsphereContext.datastore.Reference(), path)
			if linkImage {
				childDisk := devices.ChildDisk(disk)
				devices = append(devices, childDisk)
			} else {
				devices = append(devices, disk)
			}
		}

		// Mount the ISO
		{
			controller, err := devices.FindIDEController(devices.Name(isoDevice))
			if err != nil {
				return nil, fmt.Errorf("error finding iso controller: %v", err)
			}

			isoStat, err := vsphereContext.datastore.Stat(ctx, vmxIsoPath)
			if err != nil {
				return nil, fmt.Errorf("error reading iso %q: %v", vmxIsoPath, err)
			}
			glog.Infof("using iso %q: %v", vmxIsoPath, isoStat)

			cdrom, err := devices.CreateCdrom(controller)
			if err != nil {
				return nil, fmt.Errorf("error creating iso device: %v", err)
			}

			cdrom = devices.InsertIso(cdrom, vsphereContext.datastore.Path(vmxIsoPath))
			devices = append(devices, cdrom)
		}

		// Add the network
		{
			backing, err := network.EthernetCardBackingInfo(ctx)
			if err != nil {
				return nil, fmt.Errorf("error getting network info: %v", err)
			}

			device, err := object.EthernetCardTypes().CreateEthernetCard(adapter, backing)
			if err != nil {
				return nil, fmt.Errorf("error creating ethernet device %q: %v", adapter, err)
			}

			devices = append(devices, device)
		}

		// Attach devices to VM spec
		{
			deviceChange, err := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
			if err != nil {
				return nil, fmt.Errorf("error creating devices: %v", err)
			}
			vmSpec.DeviceChange = deviceChange
		}

		{
			vmxPath := fmt.Sprintf("vms/%s/%s.vmx", name, name)

			_, err := vsphereContext.datastore.Stat(ctx, vmxPath)
			if err == nil {
				dsPath := vsphereContext.datastore.Path(vmxPath)
				return nil, fmt.Errorf("vsphere vmx file %s already exists", dsPath)
			}

			vmSpec.Files = &types.VirtualMachineFileInfo{
				VmPathName: vsphereContext.datastore.Path(vmxPath), //fmt.Sprintf("[%s]", ds.Name()),
			}

			//hostSystem, err := f.HostSystemOrDefault(ctx, "")
			//if err != nil {
			//	return fmt.Errorf("error finding hostSystem: %v", err)
			//}
			//glog.Infof("using hostSystem %q", hostSystem.Reference())

			task, err := folder.CreateVM(ctx, *vmSpec, vsphereContext.resourcePool, nil)
			if err != nil {
				return nil, fmt.Errorf("error creating vm: %v", err)
			}

			info, err := task.WaitForResult(ctx, nil)
			if err != nil {
				return nil, fmt.Errorf("error from vm creation: %v", err)
			}
			glog.Infof("created vm %v", info)
			return object.NewVirtualMachine(cloud.Client.Client, info.Result.(types.ManagedObjectReference)), nil
			//
			//if cmd.on {
			//	task, err := vm.PowerOn(ctx)
			//	if err != nil {
			//		return err
			//	}
			//
			//	_, err = task.WaitForResult(ctx, nil)
			//	if err != nil {
			//		return err
			//	}
			//}

		}
	}
}

func (a *VsphereActuator) createVM(ctx context.Context, vsphereContext *VsphereContext, name string) (*object.VirtualMachine, error) {
	ova := &OvaFile{"/tmp/ova/coreos_production_vmware_ova.ova"}
	diskProvisioning := "thin"
	ipAllocationPollicy := "dhcpPolicy"
	ipProtocol := "IPv4"
	//deploymentOption := "small"
	locale := "US"

	folder, err := vsphereContext.finder.FolderOrDefault(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("error finding folder: %v", err)
	}
	glog.Infof("using folder %q", folder.Reference())

	var host *object.HostSystem

	var rawOvf []byte
	{
		r, _, err := ova.Open("*.ovf")
		if err != nil {
			return nil, err
		}
		defer r.Close()

		rawOvf, err = ioutil.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("error reading ovf data: %v", err)
		}
	}
	var envelope *ovf.Envelope
	{
		r, _, err := ova.Open("*.ovf")
		if err != nil {
			return nil, err
		}
		defer r.Close()

		envelope, err = ovf.Unmarshal(r)
		if err != nil {
			return nil, fmt.Errorf("error parsing ovf data: %v", err)
		}
	}

	glog.Infof("envelope: %v", envelope)
	//name := "Govc Virtual Appliance"
	//if envelope.VirtualSystem != nil {
	//	name = envelope.VirtualSystem.ID
	//	if envelope.VirtualSystem.Name != nil {
	//		name = *envelope.VirtualSystem.Name
	//	}
	//}

	var networkMappings []types.OvfNetworkMapping
	{
		networks := map[string]string{}

		if envelope.Network != nil {
			for _, net := range envelope.Network.Networks {
				networks[net.Name] = net.Name
			}
		}

		//for _, net := range cmd.Options.NetworkMapping {
		//	networks[net.Name] = net.Network
		//}

		for src, dst := range networks {
			if net, err := vsphereContext.finder.Network(ctx, dst); err == nil {
				networkMappings = append(networkMappings, types.OvfNetworkMapping{
					Name:    src,
					Network: net.Reference(),
				})
			}
		}
	}

	var propertyMapping []types.KeyValue
	cisp := types.OvfCreateImportSpecParams{
		DiskProvisioning:   diskProvisioning,
		EntityName:         name,
		IpAllocationPolicy: ipAllocationPollicy,
		IpProtocol:         ipProtocol,
		OvfManagerCommonParams: types.OvfManagerCommonParams{
			//DeploymentOption: deploymentOption,
			Locale: locale},
		PropertyMapping: propertyMapping,
		NetworkMapping:  networkMappings,
	}

	m := object.NewOvfManager(vsphereContext.client)
	spec, err := m.CreateImportSpec(ctx, string(rawOvf), vsphereContext.resourcePool, vsphereContext.datastore, cisp)
	if err != nil {
		return nil, err
	}
	if spec.Error != nil {
		return nil, errors.New(spec.Error[0].LocalizedMessage)
	}
	if spec.Warning != nil {
		for _, w := range spec.Warning {
			glog.Warningf("warning from ova creation: %s", w.LocalizedMessage)
		}
	}

	//if cmd.Options.Annotation != "" {
	//	switch s := spec.ImportSpec.(type) {
	//	case *types.VirtualMachineImportSpec:
	//		s.ConfigSpec.Annotation = cmd.Options.Annotation
	//	case *types.VirtualAppImportSpec:
	//		s.VAppConfigSpec.Annotation = cmd.Options.Annotation
	//	}
	//}

	lease, err := vsphereContext.resourcePool.ImportVApp(ctx, spec.ImportSpec, folder, host)
	if err != nil {
		return nil, fmt.Errorf("error importing vapp: %v", err)
	}

	abortLease := true
	defer func() {
		if abortLease {
			fault := &types.LocalizedMethodFault{
				LocalizedMessage: "unexpected error",
			}
			if err := lease.HttpNfcLeaseAbort(ctx, fault); err != nil {
				glog.Warningf("error aborting lease: %v", err)
			}
		}
	}()

	leaseInfo, err := lease.Wait(ctx)
	if err != nil {
		return nil, err
	}

	for _, device := range leaseInfo.DeviceUrl {
		for _, item := range spec.FileItem {
			if device.ImportKey != item.DeviceId {
				continue
			}

			// TODO: Would be great to copy this instead of uploading every time
			u, err := vsphereContext.client.ParseURL(device.Url)
			if err != nil {
				return nil, err
			}

			if err := a.upload(ctx, vsphereContext.client, u, ova, &item); err != nil {
				return nil, err
			}
		}
	}

	if err := lease.HttpNfcLeaseComplete(ctx); err != nil {
		return nil, fmt.Errorf("error completing upload operation: %v", err)
	}
	abortLease = false

	vmref := leaseInfo.Entity
	vm := object.NewVirtualMachine(vsphereContext.client, vmref)

	// TODO: Do we need to inject ovf env?

	return vm, nil
}

func (a *VsphereActuator) attachISO(ctx context.Context, vsphereContext *VsphereContext, vm *object.VirtualMachine) (error) {
	name := vm.Name()
	vmxIsoPath := fmt.Sprintf("vms/%s/cloud-init.iso", name)

	var cloudconfigData cloudconfigData
	cloudconfigData = a.cloudconfigData
	cloudconfigData.Hostname = name

	if err := a.uploadISO(ctx, vsphereContext, &cloudconfigData, vmxIsoPath); err != nil {
		return err
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		return fmt.Errorf("error querying devices for VM: %v", err)
	}

	cdromName := ""
	cdrom, err := devices.FindCdrom(cdromName)
	if err != nil {
		return fmt.Errorf("error finding cdrom: %v", err)
	}

	cdrom = devices.InsertIso(cdrom, vsphereContext.datastore.Path(vmxIsoPath))
	if cdrom.Connectable == nil {
		cdrom.Connectable = &types.VirtualDeviceConnectInfo{}
	}
	// Start connected otherwise cloud-init won't pick up on it
	cdrom.Connectable.StartConnected = true

	glog.Infof("attaching ISO %s to cdrom device %v", vmxIsoPath, cdrom)
	if err := vm.EditDevice(ctx, cdrom); err != nil {
		return fmt.Errorf("error attaching ISO to cdrom device: %v", err)
	}

	return nil
}

type cloudconfigData struct {
	Hostname          string   `json:"hostname,omitempty"`
	SSHAuthorizedKeys []string `json:"ssh-authorized-keys,omitempty"`
}

func (a *VsphereActuator) uploadISO(ctx context.Context, vsphereContext *VsphereContext, cloudconfigData *cloudconfigData, vmxIsoPath string) (error) {
	// Build and upload iso
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	srcdir := filepath.Join(tmpdir, "src")
	if err := os.MkdirAll(filepath.Join(srcdir, "openstack", "latest"), 0755); err != nil {
		return fmt.Errorf("error creating iso directory %q: %v", filepath.Join(srcdir, "openstack", "latest"), err)
	}

	userdataPath := filepath.Join(srcdir, "openstack", "latest", "user_data")

	cloudconfigYaml, err := yaml.Marshal(cloudconfigData)
	if err != nil {
		return fmt.Errorf("error serializing cloud config to yaml: %v", err)
	}
	userData := "#cloud-config\n\n" + string(cloudconfigYaml)
	glog.Infof("ISO cloud-config data %q", userData)

	if err = ioutil.WriteFile(userdataPath, []byte(userData), 0644); err != nil {
		return fmt.Errorf("error writing user-data file %s: %v", userdataPath, err)
	}

	//metaDataPath := filepath.Join(srcdir, "meta-data")
	//metaData := `instance-id: i-abcd1234
	//local-hostname: ovfdemo.localdomain`
	//
	//if err = ioutil.WriteFile(metaDataPath, []byte(metaData), 0644); err != nil {
	//	return fmt.Errorf("error writing meta-data file %s: %v", metaDataPath, err)
	//}

	isoPath := filepath.Join(tmpdir, "cloud-init.iso")
	if err := CreateISO(srcdir, isoPath); err != nil {
		return fmt.Errorf("error creating iso; %v", err)
	}

	opts := soap.Upload{}
	opts.Method = "PUT"
	opts.Headers = map[string]string{
		"Overwrite": "t",
	}

	err = vsphereContext.datastore.UploadFile(ctx, isoPath, vmxIsoPath, &opts)
	if err != nil {
		return fmt.Errorf("error uploading iso to vsphere: %v", err)
	}
	glog.V(2).Infof("Uploaded ISO file %s", vmxIsoPath)
	return nil
}

func (a *VsphereActuator) upload(ctx context.Context, client *vim25.Client, url *url.URL, ova *OvaFile, item *types.OvfFileItem) (error) {
	f, size, err := ova.Open(item.Path)
	if err != nil {
		return fmt.Errorf("error opening ova item %q: %v", item.Path, err)
	}
	defer f.Close()

	glog.Infof("uploading item %s", item.Path)

	opts := soap.Upload{
		ContentLength: size,
	}

	// Non-disk files (such as .iso) use the PUT method.
	// Overwrite: t header is also required in this case (ovftool does the same)
	if item.Create {
		opts.Method = "PUT"
		opts.Headers = map[string]string{
			"Overwrite": "t",
		}
	} else {
		opts.Method = "POST"
		opts.Type = "application/x-vnd.vmware-streamVmdk"
	}

	return client.Upload(f, url, &opts)
}
