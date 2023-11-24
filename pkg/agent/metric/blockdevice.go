package metric

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/toolkits/nux"
	"k8s.io/klog/v2"
)

const (
	blockDevMetricSubsystem = "blk"
	// total number of reads completed successfully
	readRequestsKey = "read_req"
	// Adjacent read requests merged in a single req.
	readMergedKey = "read_merged"
	// Total number of sectors read successfully.
	readSectorsKey = "read_sectors"
	// Total number of ms spent by all reads.
	msecReadKey = "msec_read"
	// total number of writes completed successfully.
	writeRequestsKey = "write_req"
	// Adjacent write requests merged in a single req.
	writeMergedKey = "write_merged"
	// total number of sectors written successfully.
	writeSectorsKey = "write_sectors"
	// Total number of ms spent by all writes.
	msecWriteKey = "msec_write"
	// Number of actual I/O requests currently in flight.
	iosInProgressKey = "io_inflight"
	// Amount of time during which ios_in_progress >= 1.
	msecTotalKey = "msec_total"
	// Measure of recent I/O completion time and backlog.
	msecWeightedTotalKey = "msec_weighted_total"
	// total number of discards completed successfully.
	discardRequestsKey = "discard_req"
	// Adjacent discard requests merged in a single req.
	discardMergedKey = "discard_merged"
	// total number of sectors discarded successfully.
	discardSectorsKey = "discard_sectors"
	// Total number of ms spent by all discards.
	msecDiscardKey = "msec_discard"
)

var (
	// node is Node Name; pvc is PVC name; ns is PVC namespace; dev is device path
	labelKeys = []string{"node", "pvc", "uuid", "dev"}

	// https://github.com/toolkits/nux/blob/master/iostat.go DiskStats
	readRequestsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      readRequestsKey,
		Help:      "Total number of reads completed successfully",
	}, labelKeys)

	readMergedGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      readMergedKey,
		Help:      "Adjacent read requests merged in a single req.",
	}, labelKeys)

	readSectorsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      readSectorsKey,
		Help:      "Total number of sectors read successfully.",
	}, labelKeys)

	msecReadGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      msecReadKey,
		Help:      "Total number of ms spent by all reads.",
	}, labelKeys)

	writeRequestsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      writeRequestsKey,
		Help:      "total number of writes completed successfully.",
	}, labelKeys)

	writeMergedGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      writeMergedKey,
		Help:      "Adjacent write requests merged in a single req.",
	}, labelKeys)

	writeSectorsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      writeSectorsKey,
		Help:      "total number of sectors written successfully.",
	}, labelKeys)

	msecWriteGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      msecWriteKey,
		Help:      "Total number of ms spent by all writes.",
	}, labelKeys)

	iosInProgressGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      iosInProgressKey,
		Help:      "Number of actual I/O requests currently in flight.",
	}, labelKeys)

	msecTotalGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      msecTotalKey,
		Help:      "Amount of time during which ios_in_progress >= 1.",
	}, labelKeys)

	msecWeightedTotalGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      msecWeightedTotalKey,
		Help:      "Measure of recent I/O completion time and backlog.",
	}, labelKeys)

	discardRequestsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      discardRequestsKey,
		Help:      "total number of discards completed successfully.",
	}, labelKeys)

	discardMergedGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      discardMergedKey,
		Help:      "Adjacent discard requests merged in a single req.",
	}, labelKeys)

	discardSectorsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      discardSectorsKey,
		Help:      "total number of sectors discarded successfully.",
	}, labelKeys)

	msecDiscardGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: blockDevMetricSubsystem,
		Name:      msecDiscardKey,
		Help:      "Total number of ms spent by all discards.",
	}, labelKeys)
)

func init() {
	// block device iostat
	Registry.MustRegister(readRequestsGaugeVec)
	Registry.MustRegister(readMergedGaugeVec)
	Registry.MustRegister(readSectorsGaugeVec)
	Registry.MustRegister(msecReadGaugeVec)

	Registry.MustRegister(writeRequestsGaugeVec)
	Registry.MustRegister(writeMergedGaugeVec)
	Registry.MustRegister(writeSectorsGaugeVec)
	Registry.MustRegister(msecWriteGaugeVec)

	Registry.MustRegister(discardRequestsGaugeVec)
	Registry.MustRegister(discardMergedGaugeVec)
	Registry.MustRegister(discardSectorsGaugeVec)
	Registry.MustRegister(msecDiscardGaugeVec)

	Registry.MustRegister(msecTotalGaugeVec)
	Registry.MustRegister(msecWeightedTotalGaugeVec)
	Registry.MustRegister(iosInProgressGaugeVec)
}

type blkDevMetrics struct {
	readReq, readMerged, readSectors, msecRead             prometheus.Gauge
	writeReq, writeMerged, writeSectors, msecWrite         prometheus.Gauge
	discardReq, discardMerged, discardSectors, msecDiscard prometheus.Gauge
	ioInProcess, msecTotal, msecWeightedTotal              prometheus.Gauge
}

type blockDeviceMetricWriter struct {
	diffCache
}

func NewBlockDeviceMetricWriter() *blockDeviceMetricWriter {
	return &blockDeviceMetricWriter{}
}

func (w *blockDeviceMetricWriter) name() string {
	return "blkdevice"
}

func (w *blockDeviceMetricWriter) writeMetrics(list []metricTarget) (err error) {
	var (
		diskStats []*nux.DiskStats
		// key is device id (diskStat.Major*256 + diskStat.Minor)
		devMap = make(map[uint64]metricTarget, len(list))
	)

	// 1. set device map
	for _, item := range list {
		if item.itemID.devID > 0 {
			devMap[item.itemID.devID] = item
		}
	}
	if len(devMap) == 0 {
		klog.Info("no blkdev to collect metrics")
		return
	}

	// 2. get all blockdevice stat
	diskStats, err = nux.ListDiskStats()
	if err != nil {
		err = fmt.Errorf("error of reading blkdev stats: %w", err)
		return
	}

	for _, diskStat := range diskStats {
		deviceId := uint64(diskStat.Major*256 + diskStat.Minor)
		// filter by blks
		if blk, has := devMap[deviceId]; has {
			metrics := getBlkDevMetrics(blk)
			// set real-time metrics
			metrics.readReq.Set(float64(diskStat.ReadRequests))
			metrics.readMerged.Set(float64(diskStat.ReadMerged))
			metrics.readSectors.Set(float64(diskStat.ReadSectors))
			metrics.msecRead.Set(float64(diskStat.MsecRead))

			metrics.writeReq.Set(float64(diskStat.WriteRequests))
			metrics.writeMerged.Set(float64(diskStat.WriteMerged))
			metrics.writeSectors.Set(float64(diskStat.WriteSectors))
			metrics.msecWrite.Set(float64(diskStat.MsecWrite))

			metrics.discardReq.Set(float64(diskStat.DiscardRequests))
			metrics.discardMerged.Set(float64(diskStat.DiscardMerged))
			metrics.discardSectors.Set(float64(diskStat.DiscardSectors))
			metrics.msecDiscard.Set(float64(diskStat.MsecDiscard))

			metrics.msecTotal.Set(float64(diskStat.MsecTotal))
			metrics.ioInProcess.Set(float64(diskStat.IosInProgress))
			metrics.msecWeightedTotal.Set(float64(diskStat.MsecWeightedTotal))
		}
	}

	// clean metrics
	lost := w.findDiff(list)
	for _, item := range lost {
		delBlkDevMetrics(item)
	}

	// set latest
	w.setCache(list)

	return
}

func getBlkDevMetrics(blk metricTarget) blkDevMetrics {
	return blkDevMetrics{
		readReq:     readRequestsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		readMerged:  readMergedGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		readSectors: readSectorsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		msecRead:    msecReadGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),

		writeReq:     writeRequestsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		writeMerged:  writeMergedGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		writeSectors: writeSectorsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		msecWrite:    msecWriteGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),

		discardReq:     discardRequestsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		discardMerged:  discardMergedGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		discardSectors: discardSectorsGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		msecDiscard:    msecDiscardGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),

		ioInProcess:       iosInProgressGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		msecTotal:         msecTotalGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
		msecWeightedTotal: msecWeightedTotalGaugeVec.WithLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath),
	}
}

func delBlkDevMetrics(blk metricTarget) {
	readRequestsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	readMergedGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	readSectorsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	msecReadGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)

	writeRequestsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	writeMergedGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	writeSectorsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	msecWriteGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)

	discardRequestsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	discardMergedGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	discardSectorsGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	msecDiscardGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)

	iosInProgressGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	msecTotalGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
	msecWeightedTotalGaugeVec.DeleteLabelValues(blk.nodeName, blk.pvcNSedName, blk.volUUID, blk.itemID.devPath)
}
