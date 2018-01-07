package vsphere

import (
	"os/exec"
	"runtime"
	"fmt"
	"bytes"
	"github.com/golang/glog"
)

func CreateISO(srcdir string, isoFile string) (error) {
	var commandName string

	var cmd *exec.Cmd
	switch os := runtime.GOOS; os {
	//case "darwin":
		// hdiutil makehybrid -iso -joliet -default-volume-name config-2 -o configdrive.iso /tmp/new-drive
	case "linux":
		cmd = exec.Command("genisoimage", "-o", isoFile, "-volid", "config-2", "-joliet", "-rock", srcdir)

	default:
		return fmt.Errorf("unsupported operation system for ISO generation (%s)", os)
	}


	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		glog.Errorf("Error %s occurred while executing command %+v", err, cmd)
		glog.Infof("stdout: %s", stdout.String())
		glog.Infof("stderr: %s", stderr.String())
		return fmt.Errorf("unexpected error while creating iso: %v", err)
	}

	glog.V(4).Infof("%s std output : %s\n", commandName, stdout.String())
	glog.V(4).Infof("%s std error : %s\n", commandName, stderr.String())
	return nil
}
