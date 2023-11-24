package scheduler

import (
	"sync"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/priority"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

type SchedulerIface interface {
	ScheduleVolume(allNodes []*state.Node, vol *v1.AntstorVolume) (node v1.NodeInfo, err error)
	ScheduleVolumeGroup(allNodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (err error)
}

type scheduler struct {
	// config for scheduler
	cfg config.Config
	// lock for sched
	lock sync.Mutex
}

func NewScheduler(cfg config.Config) SchedulerIface {
	return &scheduler{
		cfg: cfg,
	}
}

// ScheduleVolume return error if there is no StoragePool available
func (s *scheduler) ScheduleVolume(allNodes []*state.Node, vol *v1.AntstorVolume) (node v1.NodeInfo, err error) {
	// schedule volume one by one
	s.lock.Lock()
	defer s.lock.Unlock()

	// if volume has selected-tgt-node hint, use this node.
	// Background: In k8s scheduler-plugin mode, volume scheduling is done in Filter and Reserve phase.
	// In PreBind, plugin merges SelectedTgtNodeKey to PVC's annotation, which will be passed to Volume's annotation.
	if nodeName, has := vol.Annotations[v1.SelectedTgtNodeKey]; has {
		// ID is sufficient
		node.ID = nodeName
		klog.Infof("volume(name=%s, uuid=%s) has SelectedTgtNodeKey Annotation. assign to node %s", vol.Name, vol.UID, nodeName)
		for _, item := range allNodes {
			if item.Info.ID == nodeName {
				node = *item.Info
			}
		}
		return
	}

	// sched
	n, err := s.sched(allNodes, vol)
	if err != nil {
		return
	}
	node = n.Pool.Spec.NodeInfo
	vol.Spec.TargetNodeId = node.ID
	klog.Infof("Sched vol %s to node %s %s", vol.Name, node.ID, node.IP)

	return
}

func (s *scheduler) sched(nodes []*state.Node, vol *v1.AntstorVolume) (node *state.Node, err error) {
	nodes, err = predicate(nodes, vol, s.cfg.Scheduler)
	if err != nil {
		return
	}

	node = byPriority(nodes, vol, s.cfg.Scheduler)

	return
}

// predicate filters out qualified Nodes
func predicate(nodes []*state.Node, vol *v1.AntstorVolume, cfg config.SchedulerConfig) (qualified []*state.Node, err error) {
	qualified, err = filter.NewFilterChain(cfg).
		Input(nodes, vol).
		LoadFilterFromConfig().
		MatchAll()
	return
}

func byPriority(nodes []*state.Node, vol *v1.AntstorVolume, cfg config.SchedulerConfig) (node *state.Node) {
	if len(nodes) == 0 || vol == nil {
		return
	}

	node, _ = priority.NewPriorityCalculator(cfg).
		Input(nodes, vol).
		// AddPriorityFunc(priority.PriorityByPositionAdivce).
		// AddPriorityFunc(priority.PriorityByLeastResource).
		LoadPriorityFromConfig().
		GetFirstByScore()

	return
}
