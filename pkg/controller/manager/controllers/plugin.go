package controllers

import (
	"fmt"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/syncmeta"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	PoolReconcilerPluginCreaters   []PluginFactoryFunc
	VolumeReconcilerPluginCreaters []PluginFactoryFunc
	//
	VolumeGroupReconcilerPluginCreaters []PluginFactoryFunc
	DataControlReconcilerPluginCreaters []PluginFactoryFunc
)

func init() {
	PoolReconcilerPluginCreaters = append(PoolReconcilerPluginCreaters, NewMetaSyncerPlugin, NewLockPoolPlugin)
	VolumeReconcilerPluginCreaters = append(VolumeReconcilerPluginCreaters, NewMetaSyncerPlugin)
	VolumeGroupReconcilerPluginCreaters = append(VolumeGroupReconcilerPluginCreaters, NewMetaSyncerPlugin)
	DataControlReconcilerPluginCreaters = append(DataControlReconcilerPluginCreaters, NewMetaSyncerPlugin)
}

func RegisterPlugins(poolPlugins, volumePlugins []PluginFactoryFunc) {
	PoolReconcilerPluginCreaters = append(PoolReconcilerPluginCreaters, poolPlugins...)
	VolumeReconcilerPluginCreaters = append(VolumeReconcilerPluginCreaters, volumePlugins...)
}

type PluginHandle struct {
	Req           NewManagerRequest
	Client        client.Client
	Mgr           manager.Manager
	AntstorClient versioned.Interface
}

type PluginFactoryFunc func(h *PluginHandle) (p plugin.Plugin, err error)

// for MetaSyncerPlugin
var (
	obSyncer syncmeta.MetaSyncer
)

func NewMetaSyncerPlugin(h *PluginHandle) (p plugin.Plugin, err error) {
	if obSyncer == nil && len(h.Req.SyncDBConnInfo) > 0 {
		if len(h.Req.K8SCluster) == 0 {
			err = fmt.Errorf("K8SCluster is empty, cannot init SyncMetaService")
			klog.Error(err)
			return
		}
		obSyncer, err = syncmeta.NewOBSyncer(h.Req.K8SCluster, B64EncodedMysqlDSN(h.Req.SyncDBConnInfo), h.Client)
		if err != nil {
			klog.Error(err)
			return
		}
	}

	p = &plugin.MetaSyncPlugin{
		Syncer: obSyncer,
	}

	return
}

func NewLockPoolPlugin(h *PluginHandle) (p plugin.Plugin, err error) {
	p = &plugin.LockPoolPlugin{
		State:  h.Req.State,
		Client: h.Client,
		Cfg:    h.Req.ControllerConfig,
	}
	return
}
