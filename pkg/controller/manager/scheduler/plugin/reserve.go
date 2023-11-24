package plugin

import (
	"context"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Reserve is called by the scheduling framework when the scheduler cache is
// updated. If this method returns a failed Status, the scheduler will call
// the Unreserve method for all enabled ReservePlugins.
func (asp *AntstorSchdulerPlugin) Reserve(ctx context.Context, s *framework.CycleState, p *corev1.Pod, nodeName string) *framework.Status {
	var (
		stateNode *state.Node
		err       error
		cycledata *cycleData
	)

	// read cycledata
	cycledata, err = GetCycleData(s)
	if err != nil {
		return framework.AsStatus(err)
	}

	if cycledata.skipAntstorPlugin {
		return nil
	}

	klog.Infof("AntstorSchdulerPlugin reserve pod %s to node %s", p.Name, nodeName)

	stateNode, err = asp.State.GetNodeByNodeID(nodeName)
	if err != nil {
		return framework.AsStatus(err)
	}

	// handle MustLocal volume
	for _, pvc := range cycledata.mustLocalAntstorPVCs {
		resv := state.NewPvcReservation(pvc)
		stateNode.Reserve(resv)
		klog.Infof("AntstorSchdulerPlugin reserve %+v", resv)
		cycledata.reservations = append(cycledata.reservations, resv)
	}
	// TODO: handle other volume

	return nil
}

// Unreserve is called by the scheduling framework when a reserved pod was
// rejected, an error occurred during reservation of subsequent plugins, or
// in a later phase. The Unreserve method implementation must be idempotent
// and may be called by the scheduler even if the corresponding Reserve
// method for the same plugin was not called.
// if Bind returns Error, Unreserve will be called.
func (asp *AntstorSchdulerPlugin) Unreserve(ctx context.Context, s *framework.CycleState, p *corev1.Pod, nodeName string) {
	var (
		stateNode *state.Node
		err       error
		cycledata *cycleData
	)

	// read cycledata
	cycledata, err = GetCycleData(s)
	if err != nil {
		return
	}

	if cycledata.skipAntstorPlugin {
		return
	}

	stateNode, err = asp.State.GetNodeByNodeID(nodeName)
	if err != nil {
		return
	}

	for _, item := range cycledata.reservations {
		stateNode.Unreserve(item.ID())
	}
}
