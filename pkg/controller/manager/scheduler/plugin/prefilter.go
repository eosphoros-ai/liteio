package plugin

import (
	"context"
	"fmt"
	"sync"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

var (
	CycleDataStateKey framework.StateKey = "obnvnf/cycle-data"
)

type cycleData struct {
	// mustLocalAntstorPVCs is a list of PVC belonging to Antstor
	mustLocalAntstorPVCs []*corev1.PersistentVolumeClaim
	otherAntstorPVCs     []*corev1.PersistentVolumeClaim
	// storageclass name => sc pointer
	scMap map[string]*storagev1.StorageClass
	// skipAntstorPlugin is true then the plugin logic is skipped
	skipAntstorPlugin bool
	// reservations made in this cycle
	reservations []state.ReservationIface
	lock         sync.RWMutex
}

// Clone implements framework.StateData. framework sugguest that Clone should do a shallow copy.
func (cd *cycleData) Clone() framework.StateData {
	return &cycleData{
		otherAntstorPVCs:     cd.otherAntstorPVCs,
		mustLocalAntstorPVCs: cd.mustLocalAntstorPVCs,
		scMap:                cd.scMap,
	}
}

func GetCycleData(s *framework.CycleState) (*cycleData, error) {
	data, err := s.Read(CycleDataStateKey)
	if err != nil {
		return nil, err
	}

	if val, ok := data.(*cycleData); ok {
		return val, nil
	}

	return nil, fmt.Errorf("not found cycledata")
}

// PreFilter is called at the beginning of the scheduling cycle. All PreFilter
// plugins must return success or the pod will be rejected.
func (asp *AntstorSchdulerPlugin) PreFilter(ctx context.Context, s *framework.CycleState, pod *corev1.Pod) (*framework.PreFilterResult, *framework.Status) {
	var (
		cycleData = &cycleData{
			mustLocalAntstorPVCs: make([]*corev1.PersistentVolumeClaim, 0, len(pod.Spec.Volumes)),
			otherAntstorPVCs:     make([]*corev1.PersistentVolumeClaim, 0, len(pod.Spec.Volumes)),
			scMap:                make(map[string]*storagev1.StorageClass),
		}

		pvcCache = asp.PVCLister.PersistentVolumeClaims(pod.Namespace)
		scCache  = asp.StorageClassLister
	)

	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			// check if pvc is synced to cache
			pvcName := vol.PersistentVolumeClaim.ClaimName
			pvc, err := pvcCache.Get(pvcName)
			if err != nil {
				// return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
				return nil, framework.AsStatus(err)
			}

			// check storageclass
			scName := pvc.Spec.StorageClassName
			sc, err := scCache.Get(*scName)
			if err != nil {
				return nil, framework.AsStatus(err)
			}
			cycleData.scMap[*scName] = sc

			if isAntstorStorageClass(sc) {
				if isMustLocalStorageClass(sc) {
					cycleData.mustLocalAntstorPVCs = append(cycleData.mustLocalAntstorPVCs, pvc)
				} else {
					cycleData.otherAntstorPVCs = append(cycleData.otherAntstorPVCs, pvc)
				}
			}
		}
	}

	if len(cycleData.mustLocalAntstorPVCs) == 0 && len(cycleData.otherAntstorPVCs) == 0 {
		cycleData.skipAntstorPlugin = true
	}

	// record cycel data
	s.Write(CycleDataStateKey, cycleData)

	return nil, nil
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one,
// or nil if it does not. A Pre-filter plugin can provide extensions to incrementally
// modify its pre-processed info. The framework guarantees that the extensions
// AddPod/RemovePod will only be called after PreFilter, possibly on a cloned
// CycleState, and may call those functions more than once before calling
// Filter again on a specific node.
func (asp *AntstorSchdulerPlugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}
