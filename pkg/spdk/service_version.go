package spdk

import (
	"errors"
	"strings"
	"syscall"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/client"
	"k8s.io/klog/v2"
)

type SpdkVersionIface interface {
	Version() (ver string, err error)
}

func (svc *SpdkService) Version() (ver string, err error) {
	cli, err := svc.client()
	if err != nil {
		klog.Error("spdk client is nil, try to reconnect spdk socket")
		return
	}

	var version client.SpdkVersion
	version, err = cli.GetSpdkVersion()
	if err != nil {
		// handle broken pipe error. if sock file is re-created, client should reconnect sock file.
		if errors.Is(err, syscall.EPIPE) {
			klog.Info("found borken pipe error, reconnecting spdk service")
			errRetry := svc.Reconnect()
			klog.Info(errRetry)
		}
		return
	}
	// Version is like "SPDK v21.01.1 Stupa v0.0.8 git sha1 35c4cd3c3 - Nov 17 2022 18:43:48"
	// Suffix is like " Stupa v0.0.8"
	ver = ParseStupaVersion(version.Fields.Suffix)

	return
}

// suffix is like " Stupa v0.0.8"
func ParseStupaVersion(suffix string) (ver string) {
	suffix = strings.TrimSpace(suffix)

	parts := strings.SplitN(suffix, " ", 2)
	if len(parts) == 2 {
		ver = parts[1]
	} else {
		ver = suffix
	}

	return
}
