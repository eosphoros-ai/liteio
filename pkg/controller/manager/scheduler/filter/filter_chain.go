package filter

import (
	"context"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/config"
	"lite.io/liteio/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

type FilterContext struct {
	Ctx    context.Context
	Config config.SchedulerConfig
	Error  *MergedError
}

type PredicateFunc func(*FilterContext, *state.Node, *v1.AntstorVolume) bool

type FilterChain struct {
	nodes   []*state.Node
	vol     *v1.AntstorVolume
	filters []PredicateFunc
	ctx     *FilterContext
}

func NewFilterChain(cfg config.SchedulerConfig) *FilterChain {
	chain := &FilterChain{
		ctx: &FilterContext{
			Ctx:    context.Background(),
			Config: cfg,
			Error:  NewMergedError(),
		},
	}
	chain.WithContextValue(CtxErrKey, NewMergedError())

	return chain
}

func (fc *FilterChain) WithContextValue(key, val interface{}) *FilterChain {
	fc.ctx.Ctx = context.WithValue(fc.ctx.Ctx, key, val)
	return fc
}

func (fc *FilterChain) Input(nodes []*state.Node, vol *v1.AntstorVolume) *FilterChain {
	fc.nodes = nodes
	fc.vol = vol
	return fc
}

func (fc *FilterChain) LoadFilterFromConfig() *FilterChain {
	for _, name := range fc.ctx.Config.Filters {
		f, err := GetFilterByName(name)
		if err != nil {
			klog.Error(err)
			continue
		} else {
			klog.Info("use filter ", name)
			fc.Filter(f)
		}
	}

	return fc
}

func (fc *FilterChain) Filter(f PredicateFunc) *FilterChain {
	fc.filters = append(fc.filters, f)
	return fc
}

func (fc *FilterChain) MatchAll() (candidates []*state.Node, err error) {
	for _, node := range fc.nodes {
		if fc.passAllFilters(fc.filters, node, fc.vol) {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		err = fc.ctx.Error
	}
	return
}

func (fc *FilterChain) passAllFilters(filters []PredicateFunc, node *state.Node, vol *v1.AntstorVolume) bool {
	for _, filterFunc := range filters {
		if !filterFunc(fc.ctx, node, vol) {
			return false
		}
	}
	return true
}
