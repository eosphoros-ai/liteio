package metric

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	initiatorMetricSubsystem = "fs"

	initiatorTotalSizeBytes = "total_size_bytes"
	initiatorUsedSizeBytes  = "used_size_bytes"
	initiatorAvailSizeBytes = "avail_szie_bytes"
	initiatorTotalInodes    = "total_inodes"
	initiatorUsedInodes     = "used_inodes"
	initiatorFreeInodes     = "free_inodes"
)

var (
	initiatoeLabelKeys = []string{"node", "pvc", "ns", "initiator"}

	initiatorTotalSizeBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorTotalSizeBytes,
		Help:      "Total size of initiator FS",
	}, initiatoeLabelKeys)

	initiatorUsedSizeBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorUsedSizeBytes,
		Help:      "Used size of initiator FS",
	}, initiatoeLabelKeys)

	initiatorAvailSizeBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorAvailSizeBytes,
		Help:      "Avail size of initiator FS",
	}, initiatoeLabelKeys)

	initiatorTotalInodesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorTotalInodes,
		Help:      "Total inodes of initiator FS",
	}, initiatoeLabelKeys)

	initiatorUsedInodesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorUsedInodes,
		Help:      "Used inodes of initiator FS",
	}, initiatoeLabelKeys)

	initiatorFreeInodesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: initiatorMetricSubsystem,
		Name:      initiatorFreeInodes,
		Help:      "Free inodes of initiator FS",
	}, initiatoeLabelKeys)
)

func init() {
	Registry.MustRegister(initiatorTotalSizeBytesGaugeVec)
	Registry.MustRegister(initiatorUsedSizeBytesGaugeVec)
	Registry.MustRegister(initiatorAvailSizeBytesGaugeVec)
	Registry.MustRegister(initiatorTotalInodesGaugeVec)
	Registry.MustRegister(initiatorUsedInodesGaugeVec)
	Registry.MustRegister(initiatorFreeInodesGaugeVec)
}

func SetFilesystemMetrics(nodeID, pvcName, pvcNS string, bytes, inodes csi.VolumeUsage) {
	initiatorTotalSizeBytesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
	initiatorUsedSizeBytesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
	initiatorAvailSizeBytesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
	initiatorTotalInodesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
	initiatorUsedInodesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
	initiatorFreeInodesGaugeVec.WithLabelValues(nodeID, pvcName, pvcNS)
}

func RemoveFilesystemMetrics(nodeID, pvcName, pvcNS string) {
	initiatorTotalSizeBytesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)
	initiatorUsedSizeBytesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)
	initiatorAvailSizeBytesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)

	initiatorTotalInodesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)
	initiatorUsedInodesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)
	initiatorFreeInodesGaugeVec.DeleteLabelValues(nodeID, pvcName, pvcNS)
}
