package csi

import (
	"fmt"
	"os"
	"strings"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/metric"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi/client"
	csicmd "code.alipay.com/dbplatform/node-disk-controller/pkg/csi/csc"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi/driver"
	csimetric "code.alipay.com/dbplatform/node-disk-controller/pkg/csi/metric"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/csi/rpcserver"
	hostnvme "code.alipay.com/dbplatform/node-disk-controller/pkg/host-nvme"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/spdk/jsonrpc/nvme"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/mount"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	defaultProvisionName = "antstor.csi.alipay.com"
)

type RpcServerOption struct {
	DriverName       string
	Endpoint         string
	MaxVolume        int
	NodeID           string
	MetricListenAddr string
	// for performance profling
	PProfAddr string
	// init nvmf kernel module
	InitNvmfKernelModule bool
	IsController         bool
}

func NewCSICommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:          "antstor",
		Short:        "antstor CSI Driver",
		Long:         `antstor CSI Driver contains 2 components: controller and node-plugin`,
		SilenceUsage: false,
		Args:         cobra.NoArgs,
		Version:      fmt.Sprintf("%#v", version.Get()),
	}

	rootCmd.AddCommand(newRpcServerCmd())
	// call CSI manually
	rootCmd.AddCommand(csicmd.NewCSICommand())
	// cmd of hostnvme, for helping volume migration
	rootCmd.AddCommand(hostnvme.NewHostNvmeCommand())
	return rootCmd
}

func newRpcServerCmd() *cobra.Command {
	var opt RpcServerOption
	var cmd = &cobra.Command{
		Use:   "server",
		Short: "run rpc server of csi plugin",
		Long:  "run rpc server of csi plugin",
		Run: func(cmd *cobra.Command, args []string) {
			checkError(opt.validate())
			checkError(opt.run())
		},
	}

	cmd.Flags().StringVar(&opt.DriverName, "driver", defaultProvisionName, "the name of driver")
	cmd.Flags().StringVar(&opt.Endpoint, "endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
	cmd.Flags().StringVar(&opt.NodeID, "nodeID", "node-1", "the id of the node")
	cmd.Flags().IntVar(&opt.MaxVolume, "maxVolume", 20, "max number of volume on one node")
	cmd.Flags().StringVar(&opt.MetricListenAddr, "metricListenAddr", "", "the listen addr of metric server")
	// for controller
	cmd.Flags().BoolVar(&opt.InitNvmfKernelModule, "initKernelMod", true, "load nvmf kernel mod at starting process")
	cmd.Flags().BoolVar(&opt.IsController, "isController", false, "Run as CSI controller")

	return cmd
}

func (opt *RpcServerOption) validate() (err error) {
	return
}

// run rpcserver
func (opt *RpcServerOption) run() (err error) {
	// setup nvme module
	if opt.InitNvmfKernelModule {
		err = nvme.LoadNVMeTCP()
		if err != nil {
			klog.Error(err, " [ERROR] Failed to load nvme-tcp module. This host cannot connect NVMe target over TCP")
		}
	} else {
		klog.Info("Skipping loading nvmf kernel mod")
	}

	// init kube client
	var kubeClient *kubernetes.Clientset
	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Process is not in K8S, error is %+v", err)
	}
	cfg.UserAgent = util.KubeCfgUserAgentCSI

	if opt.IsController {
		kubeClient, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
		}
	}

	drv := driver.NewCSIDriver(driver.NewCSIDriverOption{
		Name:          opt.DriverName,
		NodeID:        opt.NodeID,
		Version:       version.Get().GitVersion,
		MaxVolume:     int64(opt.MaxVolume),
		VolumeCap:     driver.DefaultVolumeAccessModeType,
		ControllerCap: driver.DefaultControllerServiceCapability,
		NodeCap:       driver.DefaultNodeServiceCapability,
		PluginCap:     driver.DefaultPluginCapability,
	})

	cloudMgr, err := client.NewKubeAPIClient(cfg)
	if err != nil {
		klog.Fatal(err)
	}

	if opt.MetricListenAddr != "" {
		// start nodeplugin metric server
		listener, err := metric.NewListener(opt.MetricListenAddr)
		if err != nil {
			klog.Fatal(err)
		}
		go metric.NewHttpServer(csimetric.Registry).Serve(listener)
	}

	rpcserver.StartServer(opt.Endpoint, drv, mount.NewSafeMounter(), cloudMgr, kubeClient)

	return
}

func checkError(err error) {
	if err != nil {
		exit(fmt.Sprintf("%+v", err), 1)
	}
}

// fatal prints the message (if provided) and then exits. If V(2) or greater,
// klog.Fatal is invoked for extended information.
func exit(msg string, code int) {
	if klog.V(2).Enabled() {
		klog.FatalDepth(2, msg)
	}
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}
	os.Exit(code)
}
