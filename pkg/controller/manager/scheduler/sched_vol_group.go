package scheduler

import (
	"fmt"
	"math"
	"sort"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/config"
	"lite.io/liteio/pkg/controller/manager/scheduler/filter"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/util"
	"lite.io/liteio/pkg/util/misc"
	uuid "github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

// ScheduleVolume return error if there is no StoragePool available
func (s *scheduler) ScheduleVolumeGroup(allNodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (err error) {
	var (
		scheduledSize int64
		needSched     bool
		volGroupCopy  = volGroup.DeepCopy()
		qualified     []*state.Node
	)

	// check unscheduled
	for _, item := range volGroup.Spec.Volumes {
		if item.TargetNodeName != "" {
			scheduledSize += item.Size
		}
	}
	needSched = scheduledSize < volGroup.Spec.TotalSize
	if !needSched {
		return
	}

	// schedule volume one by one
	s.lock.Lock()
	defer s.lock.Unlock()

	// filter qualified nodes
	qualified, err = s.filterNodes(allNodes, volGroupCopy)
	if err != nil {
		klog.Error(err)
		return
	}

	// sort nodes by free space, large -> small
	// node usage < empty threashold, set score to 0, last of the list
	sort.Sort(sort.Reverse(SortByStorage(qualified)))

	err = schedVolGroup(s.cfg, qualified, volGroup)
	if err != nil {
		klog.Error(err)
		return
	}

	klog.Infof("Sched volGorup %s to %+v", volGroup.Name, volGroup.Spec.Volumes)

	return
}

func schedVolGroup(cfg config.Config, nodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (err error) {
	var (
		maxVolCnt            = volGroup.Spec.DesiredVolumeSpec.CountRange.Max
		maxSize              = volGroup.Spec.DesiredVolumeSpec.SizeRange.Max
		minSize              = volGroup.Spec.DesiredVolumeSpec.SizeRange.Min
		leftSize       int64 = volGroup.Spec.TotalSize
		cnt            int
		tgtNodeSet     = misc.NewEmptySet()
		unschedIndexes []int
	)

	// pick a correct size. if result is 0, it means picking failed.
	var pickSizeFn = func(node *state.Node, volSize int64) (result int64) {
		var free = *node.FreeResource.Storage()
		var picked = free
		if free.CmpInt64(volSize) > 0 {
			picked = *resource.NewQuantity(volSize, resource.BinarySI)
		}
		// TODO: if minSize=100Gi, free=1Ti, left=(1Ti+90Gi), then the next pick must fail.
		if picked.Cmp(minSize) < 0 {
			return 0
		}
		if picked.Cmp(maxSize) > 0 {
			picked = maxSize
		}

		// align to 4MiB
		bytes := int64(picked.AsApproximateFloat64())
		result = (bytes / util.FourMiB) * util.FourMiB
		if bytes%util.FourMiB > 0 {
			result += util.FourMiB
		}
		return
	}

	needSchedNextFn := func(cnt int, leftSize int64) bool {
		return cnt < maxVolCnt && leftSize > 0
	}

	// record unsched volumes
	for idx, vol := range volGroup.Spec.Volumes {
		if vol.Size > 0 && vol.TargetNodeName != "" {
			leftSize -= vol.Size
			cnt += 1
			tgtNodeSet.Add(vol.TargetNodeName)
		} else {
			unschedIndexes = append(unschedIndexes, idx)
		}
	}

	// handle unscheduled indexes
	for _, idx := range unschedIndexes {
		if needSchedNextFn(cnt, leftSize) {
			var result int64
			for _, item := range nodes {
				if !tgtNodeSet.Contains(item.Info.ID) {
					result = pickSizeFn(item, leftSize)
					// calculate allocatable bytes by min local line
					result = getAllocatableRemoteVolumeSize(item, result, float64(cfg.Scheduler.MinLocalStoragePct))
					klog.Info("getAllocatableRemoteVolumeSize picked size %d on node %s", result, item.Info.ID)

					// success
					if result > 0 {
						volGroup.Spec.Volumes[idx].Size = result
						volGroup.Spec.Volumes[idx].TargetNodeName = item.Info.ID
						tgtNodeSet.Add(item.Info.ID)
						cnt += 1
						leftSize -= result
						// TODO: check if VolId is empty?

						break
					}
				}
			}

			if result == 0 {
				// all nodes are checked, but still not scheduled
				klog.Errorf("failed to sched vol idx=%d volId=%+v", idx, volGroup.Spec.Volumes[idx].VolId)
				err = filter.NewMergedError()
				return err
			}
		}
	}

	//
	for i := 0; i < maxVolCnt-cnt; i++ {
		if needSchedNextFn(cnt, leftSize) {
			var result int64
			for _, item := range nodes {
				if !tgtNodeSet.Contains(item.Info.ID) {
					result = pickSizeFn(item, leftSize)
					// calculate allocatable bytes by min local line
					result = getAllocatableRemoteVolumeSize(item, result, float64(cfg.Scheduler.MinLocalStoragePct))
					klog.Info("getAllocatableRemoteVolumeSize picked size %d on node %s", result, item.Info.ID)

					// success
					if result > 0 {
						tgtNodeSet.Add(item.Info.ID)
						cnt += 1
						leftSize -= result

						// create volume id
						newVolId := v1.EntityIdentity{
							Namespace: volGroup.Namespace,
							Name:      fmt.Sprintf("%s-%s", volGroup.Name, misc.RandomStringWithCharSet(10, misc.LowerCharNumSet)),
							UUID:      uuid.NewV4().String(),
						}
						newVol := v1.VolumeMeta{
							VolId:          newVolId,
							Size:           result,
							TargetNodeName: item.Info.ID,
						}
						volGroup.Spec.Volumes = append(volGroup.Spec.Volumes, newVol)
						break
					}
				}
			}
			if result == 0 {
				// all nodes are checked, but still not scheduled
				klog.Errorf("failed to sched vol, leftSize=%d", leftSize)
				err = filter.NewMergedError()
				return err
			}
		}
	}

	// delete redundant volumes
	var volsCopy = make([]v1.VolumeMeta, 0, len(volGroup.Spec.Volumes))
	for _, item := range volGroup.Spec.Volumes {
		if item.Size > 0 && item.TargetNodeName != "" {
			volsCopy = append(volsCopy, item)
		}
	}
	volGroup.Spec.Volumes = volsCopy

	return
}

func (s *scheduler) filterNodes(allNodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (qualified []*state.Node, err error) {
	var (
		minSize = volGroup.Spec.DesiredVolumeSpec.SizeRange.Min
		// Here we build a fake AntstorVolume, which has minSize size and emtpy HostNode.
		// Therefore the filter only checks pool status, pool affinity in Annotation, remote volume count,
		// and SPDK condition because host node id is always different from target node id.
		// filter will make sure that pool free size is larger than minSize
		vol = &v1.AntstorVolume{
			ObjectMeta: volGroup.ObjectMeta,
			Spec: v1.AntstorVolumeSpec{
				SizeByte:       uint64(math.Round(minSize.AsApproximateFloat64())),
				HostNode:       &v1.NodeInfo{},
				PositionAdvice: v1.NoPreference,
			},
		}
	)

	// filter out unqualified nodes
	qualified, err = filter.NewFilterChain(s.cfg.Scheduler).
		Filter(func(ctx *filter.FilterContext, node *state.Node, vol *v1.AntstorVolume) bool {
			// filter empty node
			if !volGroup.Spec.Stragety.AllowEmptyNode {
				if len(node.Volumes) == 0 {
					klog.Infof("[SchedFail] volGroup=%s Pool %s, Pool is empty", volGroup.Name, node.Pool.Name)
					return false
				}
				// TODO: compare with volGroup.Spec.Stragety.EmptyThreasholdPct
			}
			return true
		}).
		Input(allNodes, vol).
		LoadFilterFromConfig().
		MatchAll()

	if len(qualified) == 0 {
		return
	}

	return
}

type SortByStorage []*state.Node

func (p SortByStorage) Len() int {
	return len(p)
}
func (p SortByStorage) Less(i, j int) bool {
	cmpStorage := p[i].FreeResource.Storage().Cmp(*p[j].FreeResource.Storage()) < 0
	// empty node
	if len(p[i].Volumes) == 0 && len(p[j].Volumes) == 0 {
		return cmpStorage
	}
	if len(p[i].Volumes) == 0 && len(p[j].Volumes) > 0 {
		return true
	}
	if len(p[i].Volumes) > 0 && len(p[j].Volumes) == 0 {
		return false
	}
	return cmpStorage
}
func (p SortByStorage) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func getAllocatableRemoteVolumeSize(node *state.Node, volSize int64, minLocalStoragePct float64) (result int64) {
	result = volSize
	if result == 0 {
		return
	}
	if node != nil {
		allocRemotes := node.GetAllocatedRemoteBytes()
		total := node.Pool.GetAvailableBytes()
		// maxResultSize := int64(float64(total)*(100-minLocalStoragePct)*100) - int64(allocRemotes)
		maxResultSize := total - int64(float64(total)*minLocalStoragePct/100) - int64(allocRemotes)
		// cannot allocate remote volume
		if maxResultSize < 0 {
			return 0
		}

		if int64(maxResultSize) < result {
			result = int64(maxResultSize)
		}
	}

	result = result / util.FourMiB * util.FourMiB
	return
}
