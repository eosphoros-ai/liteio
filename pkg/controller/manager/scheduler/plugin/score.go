package plugin

import (
	"context"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/priority"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper"
)

const (
	contextKeyCycleState = "CycleState"
)

func (asp *AntstorSchdulerPlugin) Score(ctx context.Context, stat *framework.CycleState, p *corev1.Pod, nodeName string) (int64, *framework.Status) {
	var (
		stateNode  *state.Node
		err        error
		virtualVol = &v1.AntstorVolume{}
		cycledata  *cycleData
	)

	// read cycledata
	cycledata, err = GetCycleData(stat)
	if err != nil {
		return 0, nil
	}

	if cycledata.skipAntstorPlugin {
		return 0, nil
	}

	// check if Node exists
	stateNode, err = asp.State.GetNodeByNodeID(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}

	// sumup space of PVCs(including MustLocal and PreferLocal) to virtualVol
	cycledata.lock.RLock()
	for _, pvc := range cycledata.mustLocalAntstorPVCs {
		sizeByte := int64(pvc.Spec.Resources.Requests.Storage().AsApproximateFloat64())
		virtualVol.Spec.SizeByte += uint64(sizeByte)
	}
	for _, pvc := range cycledata.otherAntstorPVCs {
		if sc, ok := cycledata.scMap[*pvc.Spec.StorageClassName]; ok {
			if sc.Parameters[v1.StorageClassParamPositionAdvice] == string(v1.PreferLocal) {
				sizeByte := int64(pvc.Spec.Resources.Requests.Storage().AsApproximateFloat64())
				virtualVol.Spec.SizeByte += uint64(sizeByte)
			}
		}
	}

	virtualVol.Spec.HostNode = &v1.NodeInfo{
		ID: nodeName,
	}

	cycledata.lock.RUnlock()

	/*
		Scoring algorithm should consider:
		1. volume's PositionAdvice
		2. if Node has plenty space for all PVs the pod is claiming, this node should rank higher
	*/
	_, score := priority.NewPriorityCalculator(asp.CustomConfig.Scheduler).
		Input([]*state.Node{stateNode}, virtualVol).
		WithContextValue(contextKeyCycleState, stat).
		LoadPriorityFromConfig().
		// AddPriorityFunc(priority.PriorityByPositionAdivce).
		// AddPriorityFunc(ScoreFuncPlentySpaceForAntstorPVCs).
		GetFirstByScore()

	return int64(score), nil
}

func (asp *AntstorSchdulerPlugin) ScoreExtensions() framework.ScoreExtensions {
	return asp
}

func (asp *AntstorSchdulerPlugin) NormalizeScore(ctx context.Context, state *framework.CycleState, p *corev1.Pod, scores framework.NodeScoreList) *framework.Status {
	return helper.DefaultNormalizeScore(framework.MaxNodeScore, false, scores)
}

func ScoreFuncPlentySpaceForAntstorPVCs(ctx context.Context, n *state.Node, vol *v1.AntstorVolume) int {
	freeBytes := int64(n.FreeResource.Storage().AsApproximateFloat64())
	if freeBytes > int64(vol.Spec.SizeByte) {
		return 50
	}

	return 0
}
