package metric

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

var metricsURLPrefix = "/metrics"

// NewListener creates a new TCP listener bound to the given address.
func NewListener(addr string) (net.Listener, error) {
	if addr == "" {
		return nil, nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		err = fmt.Errorf("metrics server failed to listen on %s, you may want to disable the metrics server or use another port if it is due to conflicts. %w", addr, err)
		klog.Error(err)
		return nil, err
	}
	klog.Infof("Metrics server is starting to listen, addr: %s", addr)
	return ln, nil
}

func NewHttpServer(registry RegistererGatherer) *http.Server {
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
	mux := http.NewServeMux()
	mux.Handle(metricsURLPrefix, handler)

	server := &http.Server{
		Handler:           mux,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadHeaderTimeout: 32 * time.Second,
	}

	return server
}
