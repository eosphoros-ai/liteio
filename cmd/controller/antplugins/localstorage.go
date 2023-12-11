package antplugin

import (
	"encoding/json"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/controllers"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
)

type LocalStorageSetting struct {
	LabelSelector          metav1.LabelSelector `json:"labelSelector" yaml:"labelSelector"`
	EnableDefault          bool                 `json:"enableDefault" yaml:"enableDefault"`
	DefaultLocalStoragePct int                  `json:"defaultLocalStoragePct" yaml:"defaultLocalStoragePct"`
}

// type AutoAdjustLocalStorageConfig struct {
// 	Enable            bool `json:"enable" yaml:"enable"`
// 	MaxCountInProcess int  `json:"maxCountInProcess" yaml:"maxCountInProcess"`
// 	// LabelSelector for matching Node labels
// 	LabelSelector metav1.LabelSelector `json:"labelSelector" yaml:"labelSelector"`
// }

type AntPluginConfigs struct {
	DefaultLocalSpaceRules []LocalStorageSetting `json:"defaultLocalSpaceRules" yaml:"defaultLocalSpaceRules"`
	// AutoAdjustLocal        AutoAdjustLocalStorageConfig `json:"autoAdjustLocal" yaml:"autoAdjustLocal"`
}

// NewReportLocalStoragePlugin returns a ReportLocalStoragePlugin
func NewReportLocalStoragePlugin(h *controllers.PluginHandle) (p plugin.Plugin, err error) {
	var pluginCfg AntPluginConfigs
	err = json.Unmarshal(h.Req.ControllerConfig.PluginConfigs, &pluginCfg)
	if err != nil {
		return
	}

	p = &ReportLocalStoragePlugin{
		NodeUpdater:        kubeutil.NewKubeNodeInfoGetter(h.Req.KubeCli),
		PoolUtil:           kubeutil.NewStoragePoolUtil(h.Client),
		ReportLocalConfigs: pluginCfg.DefaultLocalSpaceRules,
	}
	return
}

// ReportLocalStoragePlugin is a AntstorVolume plugin.
type ReportLocalStoragePlugin struct {
	// NodeGetter  kubeutil.NodeInfoGetterIface
	NodeUpdater kubeutil.NodeUpdaterIface
	PoolUtil    kubeutil.StoragePoolUpdater

	ReportLocalConfigs []LocalStorageSetting
}

func (r *ReportLocalStoragePlugin) Name() string {
	return "LocalStorageWatermark"
}

func (r *ReportLocalStoragePlugin) HandleDeletion(ctx *plugin.Context) (err error) {
	result := r.Reconcile(ctx)
	return result.Error
}

// Reconcile to apply local storage capacity to StoragePool and Node
func (r *ReportLocalStoragePlugin) Reconcile(ctx *plugin.Context) (result plugin.Result) {
	var log = ctx.Log
	var volume *v1.AntstorVolume
	var pool *v1.StoragePool
	var err error
	var isVolume, isPool bool
	var cli = ctx.Client
	var stateObj = ctx.State

	if r.NodeUpdater == nil {
		r.NodeUpdater = kubeutil.NewKubeNodeInfoGetter(ctx.KubeCli)
	}
	if r.PoolUtil == nil {
		r.PoolUtil = kubeutil.NewStoragePoolUtil(cli)
	}

	volume, isVolume = ctx.ReqCtx.Object.(*v1.AntstorVolume)
	pool, isPool = ctx.ReqCtx.Object.(*v1.StoragePool)

	if !isVolume && !isPool {
		err = fmt.Errorf("obj is not *v1.AntstorVolume or *v1.StoragePool")
		log.Error(err, "skip LocalStoragePlugin")
		return plugin.Result{}
	}

	log.Info("running LocalStoragePlugin")

	// report the local storage when the StoragePool is created in the first place.
	if isPool && pool != nil {
		totalBs := pool.GetAvailableBytes()
		if _, has := pool.Labels[v1.PoolLocalStorageBytesKey]; !has {
			log.Info("update node/status capacity", "local-storage", totalBs)
			// update Pool Label "obnvmf/node-local-storage-size" = totalBs
			err = r.PoolUtil.SavePoolLocalStorageMark(pool, uint64(totalBs))
			if err != nil {
				log.Error(err, "SavePoolLocalStorageMark failed")
				return plugin.Result{Error: err}
			}

			// update node/status capacity = totalBs
			_, err = r.NodeUpdater.ReportLocalDiskResource(pool.Name, uint64(totalBs))
			if err != nil {
				log.Error(err, "ReportLocalDiskResource failed")
				return plugin.Result{Error: err}
			}
		}

	}

	// After remote volume is scheduled, report the local storage size to Node
	if isVolume && volume != nil && volume.Spec.TargetNodeId != "" && !volume.IsLocal() {
		var node *state.Node
		node, err = stateObj.GetNodeByNodeID(volume.Spec.TargetNodeId)
		if err != nil {
			log.Error(err, "find node failed")
			return plugin.Result{Error: err}
		}
		var sp = node.Pool

		/*
			var localStorePct int
			var volInState *v1.AntstorVolume
			for _, item := range r.ReportLocalConfigs {
				selector, err := metav1.LabelSelectorAsSelector(&item.LabelSelector)
				if err != nil {
					log.Error(err, "LabelSelectorAsSelector failed", "selector", item.LabelSelector)
					continue
				}
				if selector.Matches(labels.Set(sp.Spec.NodeInfo.Labels)) && item.EnableDefault {
					localStorePct = item.DefaultLocalStoragePct
					log.Info("matched local-storage percentage", "pct", localStorePct)
				}
			}

			volInState, err = node.GetVolumeByID(volume.Spec.Uuid)
			if err == nil {
				log.Info("copy volume into state")
				*volInState = *volume
			}
		*/

		var expectLocalSize = CalculateLocalStorageCapacity(node)
		var localSizeStr = strconv.Itoa(int(expectLocalSize))
		log.Info("compare local storage size", "in label", sp.Labels[v1.PoolLocalStorageBytesKey], "expect", localSizeStr, "delTS", volume.DeletionTimestamp)
		if val := sp.Labels[v1.PoolLocalStorageBytesKey]; val != localSizeStr {
			log.Info("update node/status capacity", "local-storage", expectLocalSize)
			// update Pool Label "obnvmf/node-local-storage-size" = expectLocalSize
			err = r.PoolUtil.SavePoolLocalStorageMark(sp, expectLocalSize)
			if err != nil {
				log.Error(err, "SavePoolLocalStorageMark failed")
				return plugin.Result{Error: err}
			}

			// update node/status capacity = expectLocalSize
			_, err = r.NodeUpdater.ReportLocalDiskResource(sp.Name, expectLocalSize)
			if err != nil {
				log.Error(err, "ReportLocalDiskResource failed")
				return plugin.Result{Error: err}
			}
		}
	}

	return plugin.Result{}
}

// CalculateLocalStorageCapacity return the total bytes of local storage watermark on the Node.
// The watermark is calculated by hints of label key, PoolStaticLocalStorageSizeKey and PoolStaticLocalStoragePercentageKey
/*
TotalAvailable = total - sum(reserved vols)
AllocatedLocalStorage = TotalAvailable - sum(remote vols)
if Pool labels has watermark key, WatermarkLocalStorage = (TotalAvailable * Pct) or Static size from the label
if WatermarkLocalStorage > AllocatedLocalStorage, return size = WatermarkLocalStorage
otherwise, return size = AllocatedLocalStorage. This means LocalStorage capacity cannot be less than AllocatedLocalStorage
*/
// size 最小值 = 已分配的本地空间
// size 最大值 = 可分配空间 - 已经分配的远程空间
// 如果没有水位线标记，则使用最大值
/*
        | Available Space                                    |
---------------------------------------------------------------
Reserved| Allocated Local |  free space    |  Allocated Remote
---------------------------------------------------------------
        |         size               |
*/
func CalculateLocalStorageCapacity(n *state.Node) (size uint64) {
	// totalSize excluding reserved lvol
	var totalSize = n.Pool.GetAvailableBytes()
	var allocRemoteBytes int64

	// allocRemoteBytes = int64(n.GetAllocatedRemoteBytes())
	for _, vol := range n.Volumes {
		// remote volume
		if !vol.IsLocal() {
			if vol.DeletionTimestamp == nil {
				// volume is not deleted, sum up the size
				allocRemoteBytes += int64(vol.Spec.SizeByte)
			}
			// if there is only one last Finalizer (InStateFinalizer), the volume should be considered removed.
		}
	}

	if val := totalSize - allocRemoteBytes; val > 0 {
		size = uint64(val)
	}
	return
}
