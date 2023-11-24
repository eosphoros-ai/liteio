package hostnvme

import (
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/version"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func NewHostNvmeCommand() *cobra.Command {
	var opt HostNvmeOption
	cmd := &cobra.Command{
		Use:   "hostnvme",
		Short: "Run host-nvme-mgr",
		Long:  `Run host-nvme-mgr`,
		Run: func(cmd *cobra.Command, args []string) {
			// print version
			version.PrintVersionInfo()
			opt.Run()
		},
	}

	cmd.Flags().StringVar(&opt.NodeID, "nodeId", "", "node name in k8s")

	return cmd
}

type HostNvmeOption struct {
	NodeID string
}

func (opt HostNvmeOption) Run() {
	rt.SetLogger(zap.New(zap.UseDevMode(true), misc.ZapTimeEncoder()))

	kubeCfg := rt.GetConfigOrDie()
	kubeCfg.UserAgent = util.KubeConfigUserAgent
	antstorCli := versioned.NewForConfigOrDie(kubeCfg)

	mgr := &HostNvmeManager{
		nodeID:   opt.NodeID,
		storeCli: antstorCli,
	}

	mgr.SyncMigrationLoop(rt.SetupSignalHandler())

	klog.Info("quit host-nvme-mgr")
}
