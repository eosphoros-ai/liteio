package plugin

import (
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisterv1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

var (
	_ framework.FilterPlugin    = &AntstorSchdulerPlugin{}
	_ framework.PreFilterPlugin = &AntstorSchdulerPlugin{}
	_ framework.ScorePlugin     = &AntstorSchdulerPlugin{}
	_ framework.PreBindPlugin   = &AntstorSchdulerPlugin{}
	_ framework.ReservePlugin   = &AntstorSchdulerPlugin{}
)

type AntstorSchdulerPlugin struct {
	handle             framework.Handle
	State              state.StateIface
	PVCLister          corelisters.PersistentVolumeClaimLister
	NodeLister         corelisters.NodeLister
	StorageClassLister storagelisterv1.StorageClassLister
	// k8s client for writing
	KCli kubernetes.Interface
	// custom config
	CustomConfig config.Config
}

func (asp *AntstorSchdulerPlugin) Name() string {
	return PluginName
}

// EventsToRegister returns a series of possible events that may cause a Pod failed by this plugin schedulable.
func (asp *AntstorSchdulerPlugin) EventsToRegister() []framework.ClusterEvent {
	events := []framework.ClusterEvent{
		// Add or Update event of StorageClass will trigger moving all pods from unschedulableQ to activeQ or backoffQ
		{Resource: framework.StorageClass, ActionType: framework.Add | framework.Update},
		{Resource: framework.PersistentVolumeClaim, ActionType: framework.Add | framework.Update},
		{Resource: framework.PersistentVolume, ActionType: framework.Add | framework.Update},
		{Resource: framework.Node, ActionType: framework.Add | framework.UpdateNodeLabel},
		{Resource: framework.CSINode, ActionType: framework.Add | framework.Update},
		// other resource will be watched by dynamicInformer.
		// GVK is expected to be at least 3-folded, separated by dots.
		// <kind in plural>.<version>.<group>
		// Valid examples:
		// - foos.v1.example.com
		// - bars.v1beta1.a.b.c

		// StoragePoolGVR is not suitable to be watched by dynamicInformer, b/c it needs to be set with custom EventHandler
		// {Resource: v1.StoragePoolGVR, ActionType: framework.Add | framework.Update},
	}

	return events
}
