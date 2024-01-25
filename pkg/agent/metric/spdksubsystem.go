package metric

import (
	"fmt"

	"lite.io/liteio/pkg/spdk"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

const (
	spdkSubsystemMetricSubsystem = "spdk_subsystem"
	subsystemReadBytes           = "read_bytes_total"
	subsystemReadReqs            = "read_reqs_total"
	subsystemWriteBytes          = "write_bytes_total"
	subsystemWriteReqs           = "write_reqs_total"
	subsystemReadLatency         = "read_lat_microseconds"
	subsystemWriteLatency        = "write_lat_microseconds"
	subsystemTimeInQueue         = "time_in_queue_microseconds"
)

var (
	subsystemLabelKeys = []string{"node", "pvc", "uuid", "nqn"}

	subsystemReadBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemReadBytes,
		Help:      "Total bytes of read completed successfully",
	}, subsystemLabelKeys)

	subsystemReadReqsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemReadReqs,
		Help:      "Total requests of read completed successfully",
	}, subsystemLabelKeys)

	subsystemWriteBytesGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemWriteBytes,
		Help:      "Total bytes of write completed successfully",
	}, subsystemLabelKeys)

	subsystemWriteReqsGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemWriteReqs,
		Help:      "Total requests of write completed successfully",
	}, subsystemLabelKeys)

	subsystemReadLatGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemReadLatency,
		Help:      "Total latency of read completed successfully",
	}, subsystemLabelKeys)

	subsystemWriteLatGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemWriteLatency,
		Help:      "Total latency of write completed successfully",
	}, subsystemLabelKeys)

	subsystemTimeInQueueGaugeVec = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: spdkSubsystemMetricSubsystem,
		Name:      subsystemTimeInQueue,
		Help:      "Total time of ios in queue",
	}, subsystemLabelKeys)
)

func init() {
	Registry.MustRegister(subsystemReadBytesGaugeVec)
	Registry.MustRegister(subsystemReadReqsGaugeVec)

	Registry.MustRegister(subsystemWriteBytesGaugeVec)
	Registry.MustRegister(subsystemWriteReqsGaugeVec)

	Registry.MustRegister(subsystemReadLatGaugeVec)
	Registry.MustRegister(subsystemWriteLatGaugeVec)
	Registry.MustRegister(subsystemTimeInQueueGaugeVec)
}

type subsystemMetrics struct {
	readBytes, readReqs            prometheus.Gauge
	writeBytes, writeReqs          prometheus.Gauge
	readLat, writeLat, timeInQueue prometheus.Gauge
}

type spdkSubsystemMetricWriter struct {
	diffCache
	spdkSvc spdk.SpdkServiceIface
}

func NewSpdkSubsystemMetricWriter(spdkSvc spdk.SpdkServiceIface) (w *spdkSubsystemMetricWriter) {
	w = &spdkSubsystemMetricWriter{
		spdkSvc: spdkSvc,
	}
	return
}

func (w *spdkSubsystemMetricWriter) name() string {
	return "spdk_subsys"
}

func (w *spdkSubsystemMetricWriter) writeMetrics(list []metricTarget) (err error) {
	var (
		subsysMap = make(map[string]metricTarget, len(list))
	)

	// 1. set map
	for _, item := range list {
		if item.itemID.subsysNQN != "" {
			subsysMap[item.itemID.subsysNQN] = item
		}
	}
	if len(subsysMap) == 0 {
		klog.Info("no spdksubsys to collect metrics")
		return
	}

	// 2. get subsystem stats
	allStats, err := w.spdkSvc.GetTargetStats()
	if err != nil {
		err = fmt.Errorf("error of getting subsystem stat: %w", err)
		return
	}

	// 3. update or create metrics
	for _, stat := range allStats {
		if tgt, has := subsysMap[stat.SubsysName]; has {
			metrics := getSubsystemMetrics(tgt)

			metrics.readBytes.Set(float64(stat.BytesRead))
			metrics.readReqs.Set(float64(stat.NumReadOps))
			metrics.writeBytes.Set(float64(stat.BytesWrite))
			metrics.writeReqs.Set(float64(stat.NumWriteOps))
			metrics.readLat.Set(float64(stat.ReadLatencyTime))
			metrics.writeLat.Set(float64(stat.WriteLatencyTime))
			metrics.timeInQueue.Set(float64(stat.TimeInQueue))
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

func getSubsystemMetrics(sub metricTarget) subsystemMetrics {
	return subsystemMetrics{
		readBytes: subsystemReadBytesGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),
		readReqs:  subsystemReadReqsGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),

		writeBytes: subsystemWriteBytesGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),
		writeReqs:  subsystemWriteReqsGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),

		readLat:     subsystemReadLatGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),
		writeLat:    subsystemWriteLatGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),
		timeInQueue: subsystemTimeInQueueGaugeVec.WithLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN),
	}
}

func deleteSubsystemMetrics(sub metricTarget) {
	subsystemReadBytesGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)
	subsystemReadReqsGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)

	subsystemWriteBytesGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)
	subsystemWriteReqsGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)

	subsystemReadLatGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)
	subsystemWriteLatGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)
	subsystemTimeInQueueGaugeVec.DeleteLabelValues(sub.nodeName, sub.pvcNSedName, sub.volUUID, sub.itemID.subsysNQN)
}
