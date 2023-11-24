package agent

import (
	"time"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/manager"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent/metric"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
)

type AgentOption = manager.Option

func NewAgentCommand() *cobra.Command {
	var ao AgentOption
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run node-disk-agent",
		Long:  `Run node-disk-agent`,
		Run: func(cmd *cobra.Command, args []string) {
			// print version
			version.PrintVersionInfo()
			Run(ao)
		},
	}

	cmd.Flags().DurationVar(&ao.HeartbeatInterval, "heartbeatInterval", 20*time.Second, "heartbeat interval duration")
	cmd.Flags().StringVar(&ao.NodeID, "nodeId", "", "node name in k8s cluster")
	cmd.Flags().Int64Var(&ao.LvsSize, "lvsSize", 0, "sdpk lvstore size")
	cmd.Flags().StringVar(&ao.LvsAioFilePath, "aioFilePath", "", "file path of sdpk aio bdev")
	cmd.Flags().BoolVar(&ao.LvsMallocBdev, "mallocBdev", false, "if use malloc bdev as base bdev for spdk lvs")
	cmd.Flags().StringVar(&ao.ConfigPath, "config", "", "file path of the config")
	cmd.Flags().StringVar(&ao.MetricListenAddr, "metricListenAddr", "", "metric server listen addr")
	cmd.Flags().IntVar(&ao.MetricIntervalSec, "metricIntervalSec", 10, "the collecting interval in second of agent metrics")

	return cmd
}

func Run(opt AgentOption) {
	var err error
	var kubeClient *kubernetes.Clientset
	var storeCli *versioned.Clientset
	var poolMgr *manager.StoragePoolManager

	// init kube client
	kubeCfg := rt.GetConfigOrDie()
	kubeCfg.UserAgent = util.KubeConfigUserAgent
	kubeClient, err = kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	// init antstor client
	storeCli = versioned.NewForConfigOrDie(kubeCfg)

	// init StoragePoolManager
	poolMgr, err = manager.NewStoragePoolManager(opt, kubeClient, storeCli)
	if err != nil {
		klog.Fatal(err)
	}
	go poolMgr.Start()

	// start metric server
	listener, err := metric.NewListener(opt.MetricListenAddr)
	if err != nil {
		klog.Fatalf("listen addr %s failed: %s", opt.MetricListenAddr, err.Error())
	}
	if listener != nil {
		go metric.NewHttpServer(metric.Registry).Serve(listener)
	}

	stopChan := misc.SetupSignalHandler(func() {
		klog.Info("on exit")
	})
	<-stopChan

	err = poolMgr.Close()
	if err != nil {
		klog.Error(err)
	}
}
