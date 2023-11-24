package filter

import (
	"fmt"
	"strings"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler/filter"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"k8s.io/klog/v2"
)

func ObReplicaFilterFunc(ctx *filter.FilterContext, n *state.Node, vol *v1.AntstorVolume) bool {
	var err *filter.MergedError = ctx.Error

	// if data-holder conflict, return false
	dataConflict := isReplicaDataConflict(n.Volumes, vol.Labels[v1.VolumeDataHolderKey])
	if dataConflict {
		err.AddReason(filter.ReasonDataConflict)
		return false
	}

	return true
}

func isReplicaDataConflict(nodeVols []*v1.AntstorVolume, volDataHolder string) bool {
	if volDataHolder == "" {
		return false
	}

	for _, item := range nodeVols {
		if item.Labels != nil {
			val, has := item.Labels[v1.VolumeDataHolderKey]
			if has {
				cluster, zone, err := parseObHolder(val)
				if err != nil {
					klog.Error(err)
					return false
				}
				volCluster, volZone, err := parseObHolder(volDataHolder)
				if err != nil {
					klog.Error(err)
					return false
				}

				klog.Infof("Comparing data conflict: %s %s %s %s", cluster, volCluster, zone, volZone)
				// 相同cluster, 不同zone, 可能是数据副本，所以冲突
				if cluster == volCluster && zone != volZone {
					return true
				}
			} else {
				klog.Warningf("vol %s not having Labels with key vol-data-holder, %+v", item.Name, item.Labels)
			}
		}
	}

	return false
}

// dataHolder format is like "ob.cluster.zone"
func parseObHolder(dataHolder string) (cluster, zone string, err error) {
	subs := strings.SplitN(dataHolder, ".", 3)
	if len(subs) == 3 {
		cluster = subs[1]
		zone = subs[2]
	} else {
		err = fmt.Errorf("dataHolder is invalid: %s", dataHolder)
	}
	return
}
