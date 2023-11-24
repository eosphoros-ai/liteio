package rpcserver

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func NewGRPCServer() *gRPCServer {
	return &gRPCServer{
		stopCh: make(chan struct{}),
		once:   sync.Once{},
	}
}

// gRPCServer is a non-blocking gRPCServer
type gRPCServer struct {
	once   sync.Once
	stopCh chan struct{}
	server *grpc.Server
}

func (s *gRPCServer) Start(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	go s.serve(endpoint, ids, cs, ns)
}

func (s *gRPCServer) Wait() {
	<-s.stopCh
}

func (s *gRPCServer) Stop() {
	s.server.GracefulStop()
	s.once.Do(func() {
		close(s.stopCh)
	})
}

func (s *gRPCServer) serve(endpoint string, ids csi.IdentityServer, cs csi.ControllerServer, ns csi.NodeServer) {
	proto, addr, err := ParseAddress(endpoint)
	if err != nil {
		klog.Fatal(err.Error())
	}

	if proto == "unix" {
		if strings.HasPrefix(addr, "/") {
			addr = "/" + addr
		}
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			klog.Fatalf("Failed to remove %s, error: %s", addr, err.Error())
		}
	}

	listener, err := net.Listen(proto, addr)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logRequestResponse),
	}
	server := grpc.NewServer(opts...)
	s.server = server

	if ids != nil {
		csi.RegisterIdentityServer(server, ids)
	}
	if cs != nil {
		csi.RegisterControllerServer(server, cs)
	}
	if ns != nil {
		csi.RegisterNodeServer(server, ns)
	}

	klog.Infof("Listening for connections on address: %#v", listener.Addr())

	err = server.Serve(listener)
	klog.Error(err)
}

func logRequestResponse(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	klog.V(3).Infof("GRPC call: %s", info.FullMethod)
	klog.V(5).Infof("GRPC request: %s", protosanitizer.StripSecrets(req))
	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC error: %v", err)
	} else {
		klog.V(5).Infof("GRPC response: %s", protosanitizer.StripSecrets(resp))
	}
	return resp, err
}

// ParseAddress
func ParseAddress(addr string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(addr), "unix://") || strings.HasPrefix(strings.ToLower(addr), "tcp://") {
		s := strings.SplitN(addr, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", addr)
}
