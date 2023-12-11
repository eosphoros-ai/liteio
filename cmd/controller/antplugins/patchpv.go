package antplugin

import (
	"fmt"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/controllers"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
)

func NewPatchPVPlugin(h *controllers.PluginHandle) (p plugin.Plugin, err error) {
	p = &PatchPVPlugin{
		PvUtil: kubeutil.NewPVUppdater(h.Req.KubeCli),
	}
	return
}

type PatchPVPlugin struct {
	PvUtil kubeutil.PVFetcherUpdaterIface
}

func (p *PatchPVPlugin) Name() string {
	return "PatchPV"
}

// add targetNodeId to PV Annotation, e.g. obnvmf/pv-target-node=xxx
func (p *PatchPVPlugin) Reconcile(ctx *plugin.Context) (result plugin.Result) {
	if p.PvUtil == nil {
		return
	}

	var (
		log    = ctx.Log
		obj    = ctx.ReqCtx.Object
		volume *v1.AntstorVolume
		ok     bool
		err    error
	)

	if volume, ok = obj.(*v1.AntstorVolume); !ok {
		err = fmt.Errorf("object is not *v1.AntstorVolume")
		log.Error(err, "skip PatchPVPlugin")
		return plugin.Result{}
	}

	// get pv name from label
	var pvName string
	if val, has := volume.Labels[v1.VolumePVNameLabelKey]; has {
		pvName = val
	} else {
		pvName = volume.Name
	}

	if volume.Spec.TargetNodeId != "" {
		log.Info("patching PV", "nodeId", volume.Spec.TargetNodeId, "pvName", pvName)

		err = p.PvUtil.SetTargetNodeName(pvName, volume.Spec.TargetNodeId)
		if err != nil {
			log.Error(err, "updating PV label failed")
			return plugin.Result{
				Error: err,
			}
		}
	}

	return plugin.Result{}
}

func (p *PatchPVPlugin) HandleDeletion(ctx *plugin.Context) (err error) {
	return
}
