package pool

import (
	"fmt"
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk"
	"k8s.io/klog/v2"
)

var (
	ErrTgtUnknown = fmt.Errorf("unknown status")
)

type SpdkStatus struct {
	SpdkVersion string
	Error       error
}

type ChangedStatusPayload struct {
	Last    SpdkStatus
	Current SpdkStatus
}

type SpdkWatcher struct {
	// interval to watch status
	watchInterval time.Duration
	// spdk service
	spdk spdk.SpdkVersionIface
	// latest status read from truth
	current SpdkStatus
	// when SpdkStatus is changed, watcher will notify [last status, current status] to channel
	notifies []chan<- ChangedStatusPayload
	// signal to stop Watch
	quitChan chan struct{}
}

func (p *ChangedStatusPayload) SpdkWentDown() bool {
	return p.Last.Error == nil && p.Current.Error != nil
}

func (p *ChangedStatusPayload) SpdkBackAlive() bool {
	return p.Last.Error != ErrTgtUnknown && p.Last.Error != nil && p.Current.Error == nil
}

func NewSpdkWatcher(watchInterval time.Duration, spdk spdk.SpdkVersionIface) *SpdkWatcher {
	return &SpdkWatcher{
		watchInterval: watchInterval,
		spdk:          spdk,
		notifies:      make([]chan<- ChangedStatusPayload, 0, 2),
		quitChan:      make(chan struct{}),
		current: SpdkStatus{
			Error: ErrTgtUnknown,
		},
	}
}

// ReadStatus fetches current status by calling API
func (sw *SpdkWatcher) ReadStatus() SpdkStatus {
	var status SpdkStatus
	status.SpdkVersion, status.Error = sw.spdk.Version()
	sw.current = status
	return status
}

// Current spdk status from cache
func (sw *SpdkWatcher) Current() SpdkStatus {
	return sw.current
}

// Notify returns a message pipe
func (sw *SpdkWatcher) Notify(ch chan<- ChangedStatusPayload) {
	sw.notifies = append(sw.notifies, ch)
}

// Stop watch loop
func (sw *SpdkWatcher) Stop() {
	// cannot close twice
	close(sw.quitChan)
}

// Watch blocks to watch tgt status
func (sw *SpdkWatcher) Watch() {
	var tick = time.NewTicker(sw.watchInterval)
	for {
		select {
		case <-tick.C:
			var status SpdkStatus
			status.SpdkVersion, status.Error = sw.spdk.Version()
			if status.Error != nil {
				klog.Error(status.Error)
			}

			if status.Error != sw.current.Error {
				for _, notify := range sw.notifies {
					// send message to channel with non-block style
					payload := ChangedStatusPayload{
						Last:    sw.current,
						Current: status,
					}
					select {
					case notify <- payload:
					default:
						klog.Error("notify pipe is not writable")
					}
				}

				klog.Info("sdpk status is changed. current is ", status)
			}

			sw.current = status
		case <-sw.quitChan:
			return
		}
	}
}
