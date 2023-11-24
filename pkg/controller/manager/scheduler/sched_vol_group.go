package scheduler

import (
	"fmt"
	"sort"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	uuid "github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

var (
	fourMiB int64 = 1 << 22

	ExtraPickSizeFnMap = make(map[string]GetAllocatableVolumeSizeFn)
)

type GetAllocatableVolumeSizeFn func(node *state.Node, volSize int64) (result int64)

func RegisterVolumeGroupPickSizeFn(name string, fn GetAllocatableVolumeSizeFn) {
	ExtraPickSizeFnMap[name] = fn
}

// ScheduleVolume return error if there is no StoragePool available
func (s *scheduler) ScheduleVolumeGroup(allNodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (err error) {
	var (
		scheduledSize int64
		needSched     bool
		volGroupCopy  = volGroup.DeepCopy()
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
	qualified := s.filterNodes(allNodes, volGroupCopy)

	// sort nodes by free space, large -> small
	// node usage < empty threashold, set score to 0, last of the list
	sort.Sort(sort.Reverse(SortByStorage(qualified)))

	err = schedVolGroup(qualified, volGroup)
	if err != nil {
		return
	}

	klog.Infof("Sched volGorup %s to %+v", volGroup.Name, volGroup.Spec.Volumes)

	return
}

func schedVolGroup(nodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (err error) {
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
		result = (bytes / fourMiB) * fourMiB
		if bytes%fourMiB > 0 {
			result += fourMiB
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
					for name, extraFn := range ExtraPickSizeFnMap {
						result = extraFn(item, result)
						klog.Info("pickSize Fn %s, picked size %d on node %s", name, result, item.Info.ID)
						if result == 0 {
							break
						}
					}
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
					for name, extraFn := range ExtraPickSizeFnMap {
						result = extraFn(item, result)
						klog.Info("pickSize Fn %s, picked size %d on node %s", name, result, item.Info.ID)
						if result == 0 {
							break
						}
					}
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

func (s *scheduler) filterNodes(allNodes []*state.Node, volGroup *v1.AntstorVolumeGroup) (qualified []*state.Node) {
	var (
		minSize         = volGroup.Spec.DesiredVolumeSpec.SizeRange.Min
		maxRemoteVolCnt = s.cfg.Scheduler.MaxRemoteVolumeCount
	)

	// filter out unqualified nodes
	for _, node := range allNodes {
		// pool status
		if !node.Pool.IsSchedulable() {
			continue
		}

		// node free space < min size
		free := node.FreeResource.Storage()
		if free.Cmp(minSize) < 0 {
			continue
		}

		// node spdk unhealthy
		var spdkCond = v1.StatusError
		for _, cond := range node.Pool.Status.Conditions {
			if cond.Type == v1.PoolConditionSpkdHealth {
				spdkCond = cond.Status
			}
		}
		if spdkCond != v1.StatusOK {
			continue
		}

		// filter empty node
		if !volGroup.Spec.Stragety.AllowEmptyNode {
			freeFloat := free.AsApproximateFloat64()
			total := node.Pool.GetVgTotalBytes()
			// if node's real usage < EmptyThreasholdPct, the node is considered as empty
			if (float64(total)-freeFloat)/float64(total)*100 <= float64(volGroup.Spec.Stragety.EmptyThreasholdPct) {
				continue
			}
		}

		// remote volume count
		if node.RemoteVolumesCount(s.cfg.Scheduler.RemoteIgnoreAnnoSelector)+1 >= maxRemoteVolCnt {
			continue
		}

		qualified = append(qualified, node)
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
