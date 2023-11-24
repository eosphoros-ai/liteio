package filter

import (
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	schedcore "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
)

func AffinityFilterFunc(ctx *FilterContext, n *state.Node, vol *v1.AntstorVolume) bool {
	// consider node affinity
	if vol.Spec.NodeAffinity != nil && vol.Spec.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		// It is a little tricky to create a Node only with Labels. This is an easy way to reuse MatchNodeSelectorTerms.
		// MatchNodeSelectorTerms extracts nodeLabels and nodeFileds, and use NodeSelector to match them.
		// Code: https://github.com/kubernetes/component-helpers/blob/master/scheduling/corev1/nodeaffinity/nodeaffinity.go#L84
		match, err := schedcore.MatchNodeSelectorTerms(convertNodeInfo(n.Info), vol.Spec.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		if !match || err != nil {
			klog.Infof("[SchedFail] vol=%s Pool %s NodeAffnity fail", vol.Name, n.Pool.Name)
			return false
		}
	}
	// Reuse of K8S LabelSelector expression
	// Doc: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	if val, has := vol.Annotations[v1.NodeLabelSelectorKey]; has {
		selector, err := labels.Parse(val)
		if err != nil {
			klog.Errorf("invalid value of NodeLabelSelectorKey, %s, %+v", val, err)
		} else {
			matched := selector.Matches(labels.Set(n.Info.Labels))
			if !matched {
				klog.Infof("[SchedFail] vol=%s Pool %s NodeLabelSelector fail", vol.Name, n.Pool.Name)
				return false
			}
		}
	}

	// consider pool affinity
	if vol.Spec.PoolAffinity != nil && vol.Spec.PoolAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		match, err := schedcore.MatchNodeSelectorTerms(convertPoolLabels(n.Pool.Labels), vol.Spec.PoolAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
		if !match || err != nil {
			klog.Infof("[SchedFail] vol=%s Pool %s PoolAffinity fail", vol.Name, n.Pool.Name)
			return false
		}
	}
	if val, has := vol.Annotations[v1.PoolLabelSelectorKey]; has {
		selector, err := labels.Parse(val)
		if err != nil {
			klog.Errorf("invalid value of PoolLabelSelectorKey, %s, %+v", val, err)
		} else {
			matched := selector.Matches(labels.Set(n.Pool.Labels))
			if !matched {
				klog.Infof("[SchedFail] vol=%s Pool %s PoolLabelSelector fail", vol.Name, n.Pool.Name)
				return false
			}
		}
	}

	return true
}

func convertNodeInfo(nodeInfo *v1.NodeInfo) (node *corev1.Node) {
	node = &corev1.Node{}
	node.Name = nodeInfo.ID
	node.Labels = nodeInfo.Labels
	return
}

func convertPoolLabels(labels labels.Set) (node *corev1.Node) {
	node = &corev1.Node{}
	node.Labels = labels
	return
}
