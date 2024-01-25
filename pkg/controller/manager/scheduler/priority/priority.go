package priority

import (
	"context"
	"sort"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/config"
	"lite.io/liteio/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

type PriorityResult struct {
	NodeID string
	Score  int
}

type PriorityResultList []PriorityResult

func (p PriorityResultList) Len() int {
	return len(p)
}
func (p PriorityResultList) Less(i, j int) bool {
	return p[i].Score < p[j].Score
}
func (p PriorityResultList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

type PriorityFunc func(context.Context, *state.Node, *v1.AntstorVolume) int

type PriorityCalculator struct {
	nodes []*state.Node
	vol   *v1.AntstorVolume
	funcs []PriorityFunc
	ctx   context.Context
	cfg   config.SchedulerConfig
}

func NewPriorityCalculator(cfg config.SchedulerConfig) *PriorityCalculator {
	return &PriorityCalculator{
		ctx: context.Background(),
		cfg: cfg,
	}
}

func (pc *PriorityCalculator) Input(nodes []*state.Node, vol *v1.AntstorVolume) *PriorityCalculator {
	pc.nodes = nodes
	pc.vol = vol
	return pc
}

func (pc *PriorityCalculator) LoadPriorityFromConfig() *PriorityCalculator {
	for _, name := range pc.cfg.Priorities {
		fn, err := GetPriorityByName(name)
		if err != nil {
			klog.Error(err)
			continue
		} else {
			klog.Info("use priority ", name)
			pc.AddPriorityFunc(fn)
		}
	}

	return pc
}

func (pc *PriorityCalculator) AddPriorityFunc(f PriorityFunc) *PriorityCalculator {
	pc.funcs = append(pc.funcs, f)
	return pc
}

func (pc *PriorityCalculator) WithContextValue(key, val interface{}) *PriorityCalculator {
	pc.ctx = context.WithValue(pc.ctx, key, val)
	return pc
}

// GetFirstByScore returns the best Node and score int
func (pc *PriorityCalculator) GetFirstByScore() (*state.Node, int) {
	if len(pc.nodes) == 0 {
		return nil, 0
	}

	if pc.ctx == nil {
		pc.ctx = context.Background()
	}

	resultList := make([]PriorityResult, 0, len(pc.nodes))
	for _, node := range pc.nodes {
		var result = PriorityResult{
			NodeID: node.Pool.Spec.NodeInfo.ID,
		}
		for _, pfunc := range pc.funcs {
			result.Score += pfunc(pc.ctx, node, pc.vol)
		}
		resultList = append(resultList, result)
	}

	sort.Sort(sort.Reverse(PriorityResultList(resultList)))

	score := resultList[0].Score
	nodeID := resultList[0].NodeID
	for _, node := range pc.nodes {
		if node.Pool.Spec.NodeInfo.ID == nodeID {
			return node, score
		}
	}

	return nil, 0
}
