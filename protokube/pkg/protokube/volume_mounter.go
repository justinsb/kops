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

package protokube

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/mount"
	"os"
	"time"
)

type VolumeMountController struct {
	mounted map[string]*Volume

	provider Volumes
}

func newVolumeMountController(provider Volumes) *VolumeMountController {
	c := &VolumeMountController{}
	c.mounted = make(map[string]*Volume)
	c.provider = provider
	return c
}

func (k *VolumeMountController) mountMasterVolumes() ([]*Volume, error) {
	// TODO: mount ephemeral volumes (particular on AWS)?

	// Mount master volumes
	attached, err := k.attachMasterVolumes()
	if err != nil {
		return nil, fmt.Errorf("unable to attach master volumes: %v", err)
	}

	for _, v := range attached {
		existing := k.mounted[v.ID]
		if existing != nil {
			continue
		}

		glog.V(2).Infof("Master volume %q is attached at %q", v.ID, v.LocalDevice)

		mountpoint := "/mnt/master-" + v.ID
		glog.Infof("Doing safe-format-and-mount of %s to %s", v.LocalDevice, mountpoint)
		fstype := ""
		err = k.safeFormatAndMount(v.LocalDevice, mountpoint, fstype)
		if err != nil {
			glog.Warningf("unable to mount master volume: %q", err)
			continue
		}

		glog.Infof("mounted master volume %q on %s", v.ID, mountpoint)

		v.Mountpoint = mountpoint
		k.mounted[v.ID] = v
	}

	var volumes []*Volume
	for _, v := range k.mounted {
		volumes = append(volumes, v)
	}
	return volumes, nil
}

func (k *VolumeMountController) safeFormatAndMount(device string, mountpoint string, fstype string) error {
	// Wait for the device to show up

	for {
		_, err := os.Stat(PathFor(device))
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("error checking for device %q: %v", device, err)
		}
		glog.Infof("Waiting for device %q to be attached", device)
		time.Sleep(1 * time.Second)
	}
	glog.Infof("Found device %q", device)

	//// Mount the device
	//var mounter mount.Interface
	//runner := exec.New()
	//if k.Containerized {
	//	mounter = mount.NewNsenterMounter()
	//	runner = NewChrootRunner(runner, "/rootfs")
	//} else {
	//	mounter = mount.New()
	//}

	// If we are containerized, we still first SafeFormatAndMount in our namespace
	// This is because SafeFormatAndMount doesn't seem to work in a container
	safeFormatAndMount := &mount.SafeFormatAndMount{Interface: mount.New(""), Runner: exec.New()}

	// Check if it is already mounted
	mounts, err := safeFormatAndMount.List()
	if err != nil {
		return fmt.Errorf("error listing existing mounts: %v", err)
	}

	// Note: IsLikelyNotMountPoint is not containerized

	findMountpoint := PathFor(mountpoint)
	var existing []*mount.MountPoint
	for i := range mounts {
		m := &mounts[i]
		glog.V(8).Infof("found existing mount: %v", m)
		if m.Path == findMountpoint {
			existing = append(existing, m)
		}
	}

	options := []string{}
	//if readOnly {
	//	options = append(options, "ro")
	//}
	if len(existing) == 0 {
		glog.Infof("Creating mount directory %q", PathFor(mountpoint))
		if err := os.MkdirAll(PathFor(mountpoint), 0750); err != nil {
			return err
		}

		glog.Infof("Mounting device %q on %q", PathFor(device), PathFor(mountpoint))

		err = safeFormatAndMount.FormatAndMount(PathFor(device), PathFor(mountpoint), fstype, options)
		if err != nil {
			//os.Remove(mountpoint)
			return fmt.Errorf("error formatting and mounting disk %q on %q: %v", PathFor(device), PathFor(mountpoint), err)
		}

		// If we are containerized, we then also mount it into the host
		if Containerized {
			hostMounter := mount.NewNsenterMounter()
			err = hostMounter.Mount(device, mountpoint, fstype, options)
			if err != nil {
				//os.Remove(mountpoint)
				return fmt.Errorf("error formatting and mounting disk %q on %q in host: %v", device, mountpoint, err)
			}
		}
	} else {
		glog.Infof("Device already mounted on %q, verifying it is our device", mountpoint)

		if len(existing) != 1 {
			glog.Infof("Existing mounts unexpected")

			for i := range mounts {
				m := &mounts[i]
				glog.Infof("%s\t%s", m.Device, m.Path)
			}

			return fmt.Errorf("Found multiple existing mounts of %q at %q", device, mountpoint)
		} else {
			glog.Infof("Found existing mount of %q at %q", device, mountpoint)
		}

	}
	return nil
}

func (k *VolumeMountController) attachMasterVolumes() ([]*Volume, error) {
	volumes, err := k.provider.FindVolumes()
	if err != nil {
		return nil, err
	}

	// TODO: Only mount one of e.g. an etcd cluster?
	// Currently taken care of by zone rules though

	var tryAttach []*Volume
	var attached []*Volume
	for _, v := range volumes {
		if v.AttachedTo == "" {
			tryAttach = append(tryAttach, v)
		}
		if v.LocalDevice != "" {
			attached = append(attached, v)
		}
	}

	if len(tryAttach) != 0 {
		for _, v := range tryAttach {
			glog.V(2).Infof("Trying to mount master volume: %q", v.ID)

			err := k.provider.AttachVolume(v)
			if err != nil {
				glog.Warningf("Error attaching volume %q: %v", v.ID, err)
			} else {
				if v.LocalDevice == "" {
					glog.Fatalf("AttachVolume did not set LocalDevice")
				}
				attached = append(attached, v)
			}
		}
	}

	return attached, nil
}
