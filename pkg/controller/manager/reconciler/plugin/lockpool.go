package plugin

import (
	"context"
	"fmt"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LockPoolPlugin struct {
	State  state.StateIface
	Client client.Client
	Cfg    config.Config
}

func (p *LockPoolPlugin) Name() string {
	return "LockPool"
}

func (p *LockPoolPlugin) Reconcile(ctx *Context) (result Result) {
	var (
		log = ctx.Log
		obj = ctx.ReqCtx.Object
		ok  bool
		err error

		sp   *v1.StoragePool
		node corev1.Node
	)

	if sp, ok = obj.(*v1.StoragePool); !ok {
		err = fmt.Errorf("object is not *v1.StoragePool")
		log.Error(err, "skip LockPoolPlugin")
		return Result{}
	}

	err = p.Client.Get(ctx.ReqCtx.Ctx, client.ObjectKey{
		Name: sp.Name,
	}, &node)
	if err != nil {
		log.Error(err, "fetching node failed")
	}

	var matched bool
	// check selector
	for _, item := range p.Cfg.Scheduler.LockSchedCfg.NodeSelector {
		s := labels.NewSelector()
		op, err := convertSelectionOp(item.Operator)
		if err != nil {
			log.Error(err, "convertSelectionOp failed")
			continue
		}
		require, err := labels.NewRequirement(item.Key, op, item.Values)
		if err != nil {
			log.Error(err, "NewRequirement failed")
			continue
		}
		s = s.Add(*require)
		matched = s.Matches(labels.Set(node.Labels))
		if matched {
			log.Info("matched nodeSelector", "requirement", item, "name", sp.Name)
			break
		}
	}

	// check taints
	if !matched {
		for _, toler := range p.Cfg.Scheduler.LockSchedCfg.NodeTaints {
			matched = matchAnyTaint(toler, node.Spec.Taints)
			if matched {
				log.Info("matched nodeSelector", "toleration", toler, "name", sp.Name)
				break
			}
		}
	}

	// find the condition of KubeNode
	var cond v1.PoolCondition
	var condIdx int
	for idx, item := range sp.Status.Conditions {
		if item.Type == v1.PoolConditionKubeNode {
			cond = item
			condIdx = idx
		}
	}

	if matched && (cond.Type == "" || cond.Status == v1.StatusOK) {
		// lock pool
		log.Info("node is in NC_OFFLINE, lock storagepool", "name", sp.Name)
		if cond.Type == "" {
			sp.Status.Conditions = append(sp.Status.Conditions, v1.PoolCondition{
				Type:    v1.PoolConditionKubeNode,
				Status:  v1.StatusError,
				Message: v1.KubeNodeMsgNcOffline,
			})
		} else {
			sp.Status.Conditions[condIdx].Status = v1.StatusError
			sp.Status.Conditions[condIdx].Message = v1.KubeNodeMsgNcOffline
		}
		err = p.Client.Status().Update(ctx.ReqCtx.Ctx, sp)
		if err != nil {
			return Result{Error: err}
		}
	}

	if !matched && cond.Status == v1.StatusError {
		// unlock pool
		log.Info("node is not in NC_OFFLINE, unlock storagepool", "name", sp.Name)
		sp.Status.Conditions[condIdx].Status = v1.StatusOK
		sp.Status.Conditions[condIdx].Message = ""
		err = p.Client.Status().Update(ctx.ReqCtx.Ctx, sp)
		if err != nil {
			return Result{Error: err}
		}
	}

	return p.checkConditions(sp)
}

func (p *LockPoolPlugin) HandleDeletion(ctx *Context) (err error) {
	return
}

func convertSelectionOp(selecorOp corev1.NodeSelectorOperator) (op selection.Operator, err error) {
	switch selecorOp {
	case corev1.NodeSelectorOpIn:
		op = selection.In
	case corev1.NodeSelectorOpNotIn:
		op = selection.NotIn
	case corev1.NodeSelectorOpExists:
		op = selection.Exists
	case corev1.NodeSelectorOpDoesNotExist:
		op = selection.DoesNotExist
	case corev1.NodeSelectorOpGt:
		op = selection.GreaterThan
	case corev1.NodeSelectorOpLt:
		op = selection.LessThan
	default:
		err = fmt.Errorf("not support op %s", selecorOp)
	}

	return
}

func matchAnyTaint(toler corev1.Toleration, taints []corev1.Taint) bool {
	for _, item := range taints {
		if toler.ToleratesTaint(&item) {
			return true
		}
	}
	return false
}

func (p *LockPoolPlugin) checkConditions(sp *v1.StoragePool) (result Result) {
	var foundError bool
	for _, item := range sp.Status.Conditions {
		if item.Status == v1.StatusError {
			switch item.Type {
			case v1.PoolConditionKubeNode:
				foundError = true
			case v1.PoolConditionLvmHealth:
				foundError = true
			default:
				klog.Info("found error condition: ", item)
			}
		}
	}

	if foundError && sp.Status.Status == v1.PoolStatusReady {
		klog.Info("found node error, lock pool: ", sp.Name)
		sp.Status.Status = v1.PoolStatusLocked
		err := p.Client.Status().Update(context.Background(), sp)
		if err != nil {
			return Result{Error: err}
		}
	}

	if !foundError && sp.Status.Status == v1.PoolStatusLocked {
		klog.Info("not found node error, unlock pool: ", sp.Name)
		sp.Status.Status = v1.PoolStatusReady
		err := p.Client.Status().Update(context.Background(), sp)
		if err != nil {
			return Result{Error: err}
		}
	}

	return
}
