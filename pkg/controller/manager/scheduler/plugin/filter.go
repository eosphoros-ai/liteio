package plugin

import (
	"context"
	"math"
	"strconv"
	"strings"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const (
	NoFitStoragePool = "NoFitStoragePool"

	nodeLabelKeyHostName = "kubernetes.io/hostname"
)

// Filter is called by the scheduling framework.
// All FilterPlugins should return "Success" to declare that
// the given node fits the pod. If Filter doesn't return "Success",
// it will return "Unschedulable", "UnschedulableAndUnresolvable" or "Error".
// For the node being evaluated, Filter plugins should look at the passed
// nodeInfo reference for this particular node's information (e.g., pods
// considered to be running on the node) instead of looking it up in the
// NodeInfoSnapshot because we don't guarantee that they will be the same.
// For example, during preemption, we may pass a copy of the original
// nodeInfo object that has some pods removed from it to evaluate the
// possibility of preempting them to schedule the target pod.
func (asp *AntstorSchdulerPlugin) Filter(ctx context.Context, s *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// NOTICE: Filter will be called concurrently in multiple goroutines for multiple nodes.
	// Be careful with data race.
	var (
		stateNode *state.Node
		err       error
		node      = nodeInfo.Node()
		cycledata *cycleData
		// sumup of size of MustLocal PVC request storage
		virtualMustLocalVol v1.AntstorVolume
		// the sumup of PVC annotation key PVCAnnotationSnapshotReservedSize
		snapshotReservedSize int
	)
	klog.V(5).Infof("filter pod: %s for node %s", pod.Name, node.Name)

	// read cycledata
	cycledata, err = GetCycleData(s)
	if err != nil {
		return framework.AsStatus(err)
	}

	if cycledata.skipAntstorPlugin {
		return framework.NewStatus(framework.Success, "")
	}

	// check if Node exists
	stateNode, err = asp.State.GetNodeByNodeID(node.Name)
	if err != nil {
		return framework.NewStatus(framework.Unschedulable, NoFitStoragePool)
	}

	// if PVC is Bound, only consider this node
	for _, pvc := range cycledata.mustLocalAntstorPVCs {
		if nodeName, bound := isPVCFullyBound(pvc); bound {
			if node.Name != nodeName {
				return framework.NewStatus(framework.Unschedulable, NoFitStoragePool)
			}
		}
	}

	// Step1: classify PVCs by PositionAdvice type -> (MustLocal, Other)
	// merge MustLocal PVCs to a virtual Volume to check resource of the Node
	virtualMustLocalVol.Annotations = make(map[string]string)
	virtualMustLocalVol.Spec.HostNode = &v1.NodeInfo{
		ID:       node.Name,
		Labels:   node.Labels,
		Hostname: node.Labels[nodeLabelKeyHostName],
		IP:       node.Labels[nodeLabelKeyHostName], // TODO: fix
	}
	cycledata.lock.RLock()
	for _, pvc := range cycledata.mustLocalAntstorPVCs {
		q := pvc.Spec.Resources.Requests.Storage()
		sizeByte := int64(math.Round(q.AsApproximateFloat64()))
		virtualMustLocalVol.Spec.SizeByte += uint64(sizeByte)

		if val, has := pvc.Annotations[v1.PVCAnnotationSnapshotReservedSize]; has {
			size, err := strconv.Atoi(val)
			if err == nil {
				snapshotReservedSize += size
			}
		}
		// copy annotations
		for key, val := range pvc.Annotations {
			if strings.HasPrefix(key, "obnvmf/") {
				virtualMustLocalVol.Annotations[key] = val
			}
		}
	}
	if snapshotReservedSize > 0 {
		virtualMustLocalVol.Annotations[v1.PVCAnnotationSnapshotReservedSize] = strconv.Itoa(snapshotReservedSize)
	}
	cycledata.lock.RUnlock()

	// check if MustLocal virtual volume fits the node
	// TODO: filter need consider Reservation
	filtered, err := filter.NewFilterChain(asp.CustomConfig.Scheduler).
		Input([]*state.Node{stateNode}, &virtualMustLocalVol).
		LoadFilterFromConfig().
		// Filter(filter.BasicFilterFunc).
		// Filter(filter.AffinityFilterFunc).
		MatchAll()
	if err != nil || len(filtered) == 0 {
		return framework.NewStatus(framework.Unschedulable, NoFitStoragePool)
	}

	return framework.NewStatus(framework.Success, "")
}
