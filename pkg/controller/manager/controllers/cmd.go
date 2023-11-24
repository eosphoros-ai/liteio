package controllers

import (
	"flag"
	"fmt"
	"log"
	"os"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/agent"
	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/handler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	hostnvme "code.alipay.com/dbplatform/node-disk-controller/pkg/host-nvme"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	cligoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
)

var (
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Long:  `Print version of node-disk-controller`,
		Run: func(cmd *cobra.Command, args []string) {
			version.PrintVersionInfo()
		},
	}

	scheme = runtime.NewScheme()
)

func init() {
	// add volumev1 API to scheme
	v1.AddToScheme(scheme)
	// add built-in API to scheme
	cligoscheme.AddToScheme(scheme)
}

// TODO: WithPlugin as parameters
func NewApplicationCmd() *cobra.Command {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	klog.MaxSize = 100 * misc.MiB
	pflag.CommandLine = pflag.NewFlagSet("node-disk-controller", pflag.ExitOnError)
	// pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("v"))
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("logtostderr"))
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("log_dir"))

	// set log flag
	log.SetFlags(log.Ldate | log.LstdFlags | log.Lshortfile)

	cmd := newRootCmd()

	return cmd
}

func newRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:          "node-disk-controller",
		Short:        "node-disk-controller manages nvmf storage pool",
		Long:         `node-disk-controller manages nvmf storage pool`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Version:      fmt.Sprintf("%#v", version.Get()),
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(NewOperatorCommand())
	rootCmd.AddCommand(agent.NewAgentCommand())
	rootCmd.AddCommand(hostnvme.NewHostNvmeCommand())
	return rootCmd
}

func NewOperatorCommand() *cobra.Command {
	var option OperatorOption
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run node-disk-controller operator",
		Long:  `Run node-disk-controller operator`,
		Run: func(cmd *cobra.Command, args []string) {
			// print version
			version.PrintVersionInfo()

			option.Run()
		},
	}

	cmd.Flags().IntVar(&option.WebhookPort, "port", 9443, "webhook service port")
	cmd.Flags().StringVar(&option.MetricsAddr, "metricsAddr", ":9090", "metrics serivce address")
	cmd.Flags().StringVar(&option.HealthProbeAddr, "probeAddr", ":9080", "health serivce address")
	cmd.Flags().BoolVar(&option.DevMode, "dev", true, "log use dev mode")
	cmd.Flags().StringVar(&option.SyncDBConnInfo, "obConnInfo", "", "DB connection info for syncing meta data")
	cmd.Flags().StringVar(&option.K8SCluster, "k8sCluster", "", "Name of k8s cluster")
	cmd.Flags().StringVar(&option.ConfigPath, "config", "/controller-config.yaml", "config file path, default is /controller-config.yaml")
	cmd.Flags().StringVar(&option.WebhookTLSDir, "tlsdir", "", "dir of tls.key and tls.crt")
	cmd.Flags().BoolVar(&option.EnableWebhook, "enableWebhook", false, "enable webhook service")
	// cmd.Flags().StringVar(&co.KubeAPIURL, "kubeApiUrl", "", "APIServer URL")
	// cmd.Flags().StringVar(&co.KubeConfigPath, "kubeConfigPath", "", "file path of kube config")
	return cmd
}

type OperatorOption struct {
	WebhookPort   int
	WebhookTLSDir string
	// The address the metric endpoint binds to
	MetricsAddr string

	// Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
	HealthProbeAddr string
	// SyncDBConnInfo is a base64 encoded MySQL connection DSN
	// DSN format is like USER:PASSWORD@tcp(DOMAIN_ADDRESS:2883)/DB_NAME?charset=utf8
	SyncDBConnInfo string
	K8SCluster     string

	ConfigPath string

	// The address the probe endpoint binds to
	EnableLeaderElection bool
	// zap logger set DevMode to true
	DevMode bool
	// enable webhook service
	EnableWebhook bool
}

func (o *OperatorOption) Run() {
	kubeCfg := rt.GetConfigOrDie()
	kubeCfg.UserAgent = util.KubeConfigUserAgent
	kubeCfg.ContentType = "application/json"

	kubeClient, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	var cfg config.Config
	cfg, err = config.Load(o.ConfigPath)
	if err != nil {
		klog.Fatalf("load config failed: %s", err.Error())
	}
	config.SetDefaults(&cfg)

	klog.Infof("use config: %+v", cfg)

	req := NewManagerRequest{
		MetricsAddr:     o.MetricsAddr,
		HealthProbeAddr: o.HealthProbeAddr,
		SyncDBConnInfo:  o.SyncDBConnInfo,
		K8SCluster:      o.K8SCluster,
		WebhookPort:     o.WebhookPort,
		WebhookTLSDir:   o.WebhookTLSDir,
		EnableWebhook:   o.EnableWebhook,
		KubeConfig:      kubeCfg,
		KubeCli:         kubeClient,
		Scheme:          scheme,
		State:           state.NewState(),

		ControllerConfig: cfg,
	}
	mgr := NewAndInitControllerManager(req)

	ctx := rt.SetupSignalHandler()

	// create NodeInformer to sync nodes to cache
	nodeInformer, err := mgr.GetCache().GetInformer(ctx, &corev1.Node{})
	if err != nil {
		klog.Fatal(err)
	}
	nodeHandler := &handler.NodeEventHandler{
		Cfg: cfg,
	}
	nodeInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nodeHandler.FilterObject,
		Handler:    nodeHandler,
	})

	go func() {
		klog.Info("manager start working")
		if err := mgr.Start(ctx); err != nil {
			klog.Error(err, "manager Start failed")
			os.Exit(1)
		}
	}()

	klog.Info("manager wait for cache to be synced")
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		klog.Error(nil, "manager cache WaitForCacheSync failed")
		os.Exit(1)
	}
	klog.Info("cache is synced")

	<-ctx.Done()
}
