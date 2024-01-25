package hostnvme

import (
	"lite.io/liteio/pkg/util/misc"
	"lite.io/liteio/pkg/version"
	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"lite.io/liteio/pkg/generated/clientset/versioned"
	"lite.io/liteio/pkg/util"
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
