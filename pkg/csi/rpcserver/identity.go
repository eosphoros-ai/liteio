package rpcserver

import (
	"lite.io/liteio/pkg/csi/driver"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

type IdentityServer struct {
	driver *driver.CSIDriver
}

// NewIdentityServer creates an identity server
func NewIdentityServer(driver *driver.CSIDriver) *IdentityServer {
	return &IdentityServer{
		driver: driver,
	}
}

var _ csi.IdentityServer = &IdentityServer{}

// Probe check rpcserver is alive
func (is *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{Value: true},
	}, nil
}

// GetPluginCapabilities gets plugin capabilities: CONTROLLER, ACCESSIBILITY, EXPANSION
func (d *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.
	GetPluginCapabilitiesResponse, error) {
	klog.V(5).Infof("Using default capabilities")
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: d.driver.GetPluginCapability(),
	}, nil
}

// GetPluginInfo returns plugin name and version
func (d *IdentityServer) GetPluginInfo(ctx context.Context,
	req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	klog.V(5).Infof("Using GetPluginInfo")
	return &csi.GetPluginInfoResponse{
		Name:          d.driver.GetName(),
		VendorVersion: d.driver.GetVersion(),
	}, nil
}
