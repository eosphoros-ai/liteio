package v1

import (
	"fmt"
	"strconv"

	"k8s.io/klog/v2"
)

// GetTotalSize gets volume size + reserved snapshot size
func (vol *AntstorVolume) GetTotalSize() (size uint64) {
	var reservedSnapSize int
	var allocatedSize int
	var err error
	if vol.Annotations != nil {
		if val, has := vol.Annotations[SnapshotReservedSpaceAnnotationKey]; has {
			reservedSnapSize, err = strconv.Atoi(val)
			if err != nil {
				klog.Error(err)
			}
		}

		if val, has := vol.Annotations[AllocatedSizeAnnoKey]; has {
			allocatedSize, err = strconv.Atoi(val)
			if err != nil {
				klog.Error(err)
			}
		}
	}

	if allocatedSize > 0 {
		size = uint64(allocatedSize + reservedSnapSize)
	} else {
		size = vol.Spec.SizeByte + uint64(reservedSnapSize)
	}
	return
}

func (vol *AntstorVolume) IsLocal() bool {
	return vol.Spec.HostNode.ID == vol.Spec.TargetNodeId
}

func (vol *AntstorVolume) ReservationID() string {
	if vol.Annotations != nil {
		return vol.Annotations[ReservationIDKey]
	}

	return ""
}

func (vol *SpdkLvol) FullName() string {
	return fmt.Sprintf("%s/%s", vol.LvsName, vol.Name)
}
