package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// PreBind is called before binding a pod. All prebind plugins must return
// success or the pod will be rejected and won't be sent for binding.
// NOTICE: volumebinding plugin also implements PreBind. When PreBind is done, PVC
func (asp *AntstorSchdulerPlugin) PreBind(ctx context.Context, s *framework.CycleState, pod *corev1.Pod, nodeName string) *framework.Status {
	var (
		err       error
		cycledata *cycleData
		nodeInfo  *framework.NodeInfo
	)

	nodeInfo, err = asp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	klog.Infof("AntstorSchdulerPlugin prebind node info: %s", nodeInfo.Node().Name)

	// read cycledata
	cycledata, err = GetCycleData(s)
	if err != nil {
		return framework.AsStatus(err)
	}

	klog.Infof("reservations are %+v , skipPlugin=%t", cycledata.reservations, cycledata.skipAntstorPlugin)

	if cycledata.skipAntstorPlugin {
		return framework.NewStatus(framework.Success, "")
	}

	var bindErr error
	for _, resv := range cycledata.reservations {
		klog.Infof("to prebind Reservation %+v", resv)

		pvcName := resv.NamespacedName()
		ns, name, err := cache.SplitMetaNamespaceKey(pvcName)
		if err != nil {
			klog.Error("invalid PVC name", pvcName)
			continue
		}

		// set pvc annotation
		annotations := map[string]string{
			v1.SelectedTgtNodeKey: nodeName,
			v1.ReservationIDKey:   resv.ID(),
		}
		annoBytes, _ := json.Marshal(annotations)
		patch := fmt.Sprintf(`{"metadata": {"annotations": %s}}`, string(annoBytes))

		_, bindErr = asp.KCli.CoreV1().PersistentVolumeClaims(ns).Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
		if bindErr != nil {
			klog.Error("bind antstor PVC to node failed: ", bindErr)
			return framework.AsStatus(bindErr)
		}
	}

	return framework.NewStatus(framework.Success, "")
}
