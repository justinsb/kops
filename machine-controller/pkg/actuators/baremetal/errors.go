package baremetal

import (
	clusterv1 "k8s.io/kube-deploy/cluster-api/api/cluster/v1alpha1"
	apierrors "k8s.io/kube-deploy/cluster-api-gcp/errors"
	"github.com/golang/glog"
)

// If the GCEClient has a client for updating Machine objects, this will set
// the appropriate reason/message on the Machine.Status. If not, such as during
// cluster installation, it will operate as a no-op. It also returns the
// original error for convenience, so callers can do "return handleMachineError(...)".
func (a *BaremetalActuator) handleMachineError(machine *clusterv1.Machine, err *apierrors.MachineError) error {
	if a.machineClient != nil {
		reason := err.Reason
		message := err.Message
		machine.Status.ErrorReason = &reason
		machine.Status.ErrorMessage = &message

		// TODO: We should never use Update, always Patch
		a.machineClient.Update(machine)
	}

	glog.Errorf("Machine error: %v", err.Message)
	return err
}

