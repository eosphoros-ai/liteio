package metric

import (
	"context"
	"sync"
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/klog/v2"
)

type metricWriter interface {
	name() string
	writeMetrics(tgts []metricTarget) (err error)
}

type Collector struct {
	interval time.Duration
	lister   MetricTargetListerIface
	writers  []metricWriter
}

func NewCollector(interval time.Duration, lister MetricTargetListerIface, spdkSvc spdk.SpdkServiceIface) *Collector {
	writers := []metricWriter{
		NewSpdkLvolMetricWriter(spdkSvc),
		NewSpdkSubsystemMetricWriter(spdkSvc),
		NewBlockDeviceMetricWriter(),
	}

	return &Collector{
		interval: interval,
		lister:   lister,
		writers:  writers,
	}
}

func (c *Collector) Start(ctx context.Context) (err error) {
	ticker := time.NewTicker(c.interval)
	wg := sync.WaitGroup{}
	for {
		select {
		case <-ticker.C:
			targets := c.lister.List()
			if len(targets) > 0 {
				start := time.Now()
				for _, w := range c.writers {
					wg.Add(1)
					go func(writer metricWriter) {
						errW := writer.writeMetrics(targets)
						if errW != nil {
							klog.Error(errW, writer.name())
						}
						wg.Done()
					}(w)
				}
				wg.Wait()
				klog.Infof("collecting metrics cost time %s", time.Since(start))
			}
		case <-ctx.Done():
			klog.Info("collector quit")
			return
		}
	}
}

type diffCache struct {
	lastMap map[string]metricTarget
}

func (c *diffCache) setCache(list []metricTarget) {
	if c.lastMap == nil {
		c.lastMap = make(map[string]metricTarget, len(list))
	}
	for _, item := range list {
		c.lastMap[item.id()] = item
	}
}

func (c *diffCache) findDiff(latest []metricTarget) (lost []metricTarget) {
	if c.lastMap == nil {
		c.lastMap = make(map[string]metricTarget, len(latest))
		return
	}

	var newIdSet = misc.NewEmptySet()
	for _, item := range latest {
		newIdSet.Add(item.id())
	}

	for id, item := range c.lastMap {
		if !newIdSet.Contains(id) {
			lost = append(lost, item)
		}
	}

	return
}
