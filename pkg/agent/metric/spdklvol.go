package metric

import (
	"fmt"

	"lite.io/liteio/pkg/spdk"
	"lite.io/liteio/pkg/spdk/jsonrpc/client"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

const (
	spdkLvolMetricSubsystem = "spdk_lvol"

	lvolReadBytes      = "read_bytes_total"
	lvolReadReqs       = "read_reqs_total"
	lvolWriteBytes     = "write_bytes_total"
	lvolWriteReqs      = "write_reqs_total"
	lvolDiscardBytes   = "discard_bytes_total"
	lvolDiscardReqs    = "discard_reqs_total"
	lvolReadLatency    = "read_lat_microseconds"
	lvolWriteLatency   = "write_lat_microseconds"
	lvolDiscardLatency = "discard_lat_microseconds"
	lvolTimeInQueue    = "time_in_queue_microseconds"

	// lvolRWIopsLimit         = "readwrite_iops_limit"
	// lvolRWBandWidthLimit    = "readwrite_mbytes_per_sec_limit"
	// lvolReadBandWidthLimit  = "read_mbytes_per_sec_limit"
	// lvolWriteBadnWidthLimit = "write_mbytes_per_sec_limit"

	// lvolLvsTotalSize = "lvs_total_size_bytes"
	// lvolLvsFreeSize  = "lvs_free_size_bytes"
)

var (
	lvolLabelKeys = []string{"node", "pvc", "uuid", "lvol"}
	// lvsLabelKeys  = []string{"node", "lvs"}

	lvolReadBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolReadBytes,
		Help:      "Total bytes of read completed successfully",
	}, lvolLabelKeys)

	lvolReadReqsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolReadReqs,
		Help:      "Total requests of read completed successfully",
	}, lvolLabelKeys)

	lvolWriteBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolWriteBytes,
		Help:      "Total bytes of write completed successfully",
	}, lvolLabelKeys)

	lvolWriteReqsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolWriteReqs,
		Help:      "Total requests of write completed successfully",
	}, lvolLabelKeys)

	lvolDiscardBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolDiscardBytes,
		Help:      "Total bytes of discard completed successfully",
	}, lvolLabelKeys)

	lvolDiscardReqsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolDiscardReqs,
		Help:      "Total requests of discard completed successfully",
	}, lvolLabelKeys)

	lvolReadLatGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolReadLatency,
		Help:      "Total latency of read completed successfully",
	}, lvolLabelKeys)

	lvolWriteLatGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolWriteLatency,
		Help:      "Total latency of write completed successfully",
	}, lvolLabelKeys)

	lvolDiscardLatGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolDiscardLatency,
		Help:      "Total latency of discard completed successfully",
	}, lvolLabelKeys)

	lvolTimeInQueueGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkLvolMetricSubsystem,
		Name:      lvolTimeInQueue,
		Help:      "Total time of ios in queue",
	}, lvolLabelKeys)

	// lvolRWIopsLimitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolRWIopsLimit,
	// 	Help:      "Readwrite iops limit",
	// }, lvolLabelKeys)

	// lvolRWBandWidthLimitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolRWBandWidthLimit,
	// 	Help:      "Readwrite bandwidth limit, mbytes per sec",
	// }, lvolLabelKeys)

	// lvolReadBandWidthLimitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolReadBandWidthLimit,
	// 	Help:      "Read bandwidth limit, mbytes per sec",
	// }, lvolLabelKeys)

	// lvolWriteBandWidthLimitGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolWriteBadnWidthLimit,
	// 	Help:      "Write bandwidth limit, mbytes per sec",
	// }, lvolLabelKeys)

	// lvolLvsTotalSizeGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolLvsTotalSize,
	// 	Help:      "Total size of lvs",
	// }, lvsLabelKeys)

	// lvolLvsFreeSizeGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Subsystem: spdkLvolMetricSubsystem,
	// 	Name:      lvolLvsFreeSize,
	// 	Help:      "Free size of lvs",
	// }, lvsLabelKeys)
)

func init() {
	Registry.MustRegister(lvolReadBytesGaugeVec)
	Registry.MustRegister(lvolReadReqsGaugeVec)
	Registry.MustRegister(lvolWriteBytesGaugeVec)
	Registry.MustRegister(lvolWriteReqsGaugeVec)
	Registry.MustRegister(lvolDiscardBytesGaugeVec)
	Registry.MustRegister(lvolDiscardReqsGaugeVec)

	Registry.MustRegister(lvolReadLatGaugeVec)
	Registry.MustRegister(lvolWriteLatGaugeVec)
	Registry.MustRegister(lvolDiscardLatGaugeVec)
	Registry.MustRegister(lvolTimeInQueueGaugeVec)

	// Registry.MustRegister(lvolRWIopsLimitGaugeVec)
	// Registry.MustRegister(lvolRWBandWidthLimitGaugeVec)
	// Registry.MustRegister(lvolReadBandWidthLimitGaugeVec)
	// Registry.MustRegister(lvolWriteBandWidthLimitGaugeVec)

	// Registry.MustRegister(lvolLvsTotalSizeGaugeVec)
	// Registry.MustRegister(lvolLvsFreeSizeGaugeVec)
}

type spdkLvolMetrics struct {
	readBytes, readReqs                        prometheus.Gauge
	writeBytes, writeReqs                      prometheus.Gauge
	discardBytes, discardReqs                  prometheus.Gauge
	readLat, writeLat, discardLat, timeInQueue prometheus.Gauge
	// rwIops, rwBw, readBw, writeBw              prometheus.Gauge
}

type spdkLvolMetricWriter struct {
	diffCache
	spdkSvc spdk.SpdkServiceIface
	// spdkLvols map[string]spdkLvol // key: lvsName/itemID.lvolFullName
}

func NewSpdkLvolMetricWriter(spdkSvc spdk.SpdkServiceIface) *spdkLvolMetricWriter {
	w := &spdkLvolMetricWriter{
		spdkSvc: spdkSvc,
	}

	return w
}

func (w *spdkLvolMetricWriter) name() string {
	return "spdk_lvol"
}

func (w *spdkLvolMetricWriter) writeMetrics(list []metricTarget) (err error) {
	var (
		bdevs      []client.Bdev
		uuid2Alias = make(map[string]string)
		iostats    client.BdevIostats
		lvolMap    = make(map[string]metricTarget, len(list))
	)

	// 1. set map
	for _, item := range list {
		if item.itemID.lvolFullName != "" {
			lvolMap[item.itemID.lvolFullName] = item
		}
	}
	if len(lvolMap) == 0 {
		klog.Info("no spdklvol to collect metrics")
		return
	}

	// 2. get lvol metadata
	bdevs, err = w.spdkSvc.BdevGetBdevs(client.BdevGetBdevsReq{})
	if err != nil {
		err = fmt.Errorf("error of listing bdev metadata: %w", err)
		return
	}
	for _, bdev := range bdevs {
		// filter irrelevant bdevs, and record uuid <-> alias mapper
		if len(bdev.Aliases) == 0 {
			continue
		}
		// TODO: double check
		uuid2Alias[bdev.UUID] = bdev.Aliases[0]

		// update or create metrics
		// metrics := getSpdkLvolMetrics(lvol)
		// metrics.rwIops.Set(float64(bdev.RateLimits.RWIops))
		// metrics.rwBw.Set(float64(bdev.RateLimits.RWMbytes))
		// metrics.readBw.Set(float64(bdev.RateLimits.RMbytes))
		// metrics.writeBw.Set(float64(bdev.RateLimits.WMbytes))
	}

	// 3. get lvol stats
	iostats, err = w.spdkSvc.BdevGetIostat(client.BdevGetIostatReq{})
	if err != nil {
		err = fmt.Errorf("error of getting lvol stat: %w", err)
		return
	}

	for _, iostat := range iostats.Bdevs {
		// iostat.Name is uuid of bdev
		alias := uuid2Alias[iostat.Name]
		// filter irrelevant bdevs
		if val, has := lvolMap[alias]; has {
			metrics := getSpdkLvolMetrics(val)

			metrics.readBytes.Set(float64(iostat.BytesRead))
			metrics.readReqs.Set(float64(iostat.NumReadOps))
			metrics.writeBytes.Set(float64(iostat.BytesWritten))
			metrics.writeReqs.Set(float64(iostat.NumWriteOps))
			metrics.discardBytes.Set(float64(iostat.BytesUnmapped))
			metrics.discardReqs.Set(float64(iostat.NumUnmapOps))

			metrics.readLat.Set(float64(iostat.ReadLatencyTicks * 1000000 / iostats.TickRate))
			metrics.writeLat.Set(float64(iostat.WriteLatencyTicks * 1000000 / iostats.TickRate))
			metrics.discardLat.Set(float64(iostat.UnmapLatencyTicks * 1000000 / iostats.TickRate))
			metrics.timeInQueue.Set(float64(iostat.TimeInQueue * 1000000 / iostats.TickRate))
		}
	}

	// 4. get lvs metadata
	// lvs, err := p.spdkSvc.GetLVStore(spdk.LVStoreName)
	// if err != nil {
	// 	err = fmt.Errorf("error when getting lvs metadata: %w", err)
	// 	klog.Error(err)
	// 	continue
	// }

	// totalSize := lvolLvsTotalSizeGaugeVec.WithLabelValues(p.nodeName, lvs.Name)
	// freeSize := lvolLvsFreeSizeGaugeVec.WithLabelValues(p.nodeName, lvs.Name)
	// totalSize.Set(float64(lvs.TotalDataClusters * lvs.ClusterSize))
	// freeSize.Set(float64(lvs.FreeClusters * lvs.ClusterSize))

	// clean metrics
	lost := w.findDiff(list)
	for _, item := range lost {
		delBlkDevMetrics(item)
	}

	// set latest
	w.setCache(list)

	return
}

func getSpdkLvolMetrics(lvol metricTarget) spdkLvolMetrics {
	return spdkLvolMetrics{
		readBytes:    lvolReadBytesGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		readReqs:     lvolReadReqsGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		writeBytes:   lvolWriteBytesGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		writeReqs:    lvolWriteReqsGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		discardBytes: lvolDiscardBytesGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		discardReqs:  lvolDiscardReqsGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),

		readLat:     lvolReadLatGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		writeLat:    lvolWriteLatGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		discardLat:  lvolDiscardLatGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),
		timeInQueue: lvolTimeInQueueGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName),

		// rwIops:  lvolRWIopsLimitGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName),
		// rwBw:    lvolRWBandWidthLimitGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName),
		// readBw:  lvolReadBandWidthLimitGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName),
		// writeBw: lvolWriteBandWidthLimitGaugeVec.WithLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName),
	}
}

func deleteSpdkLvolMetrics(lvol metricTarget) {
	lvolReadBytesGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolReadReqsGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolWriteBytesGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolWriteReqsGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolDiscardBytesGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolDiscardReqsGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolReadLatGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolWriteLatGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolDiscardLatGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)
	lvolTimeInQueueGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.volUUID, lvol.itemID.lvolFullName)

	// lvolRWIopsLimitGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName)
	// lvolRWBandWidthLimitGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName)
	// lvolReadBandWidthLimitGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName)
	// lvolWriteBandWidthLimitGaugeVec.DeleteLabelValues(lvol.nodeName, lvol.pvcNSedName, lvol.pvcNS, lvol.itemID.lvolFullName)
}
