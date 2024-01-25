package plugin

import (
	"errors"

	v1 "lite.io/liteio/pkg/api/volume.antstor.alipay.com/v1"
	"lite.io/liteio/pkg/controller/manager/config"
	"lite.io/liteio/pkg/controller/manager/controllers"
	"lite.io/liteio/pkg/controller/manager/reconciler/handler"
	"lite.io/liteio/pkg/controller/manager/state"
	"lite.io/liteio/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	PluginName = "AntstorPlugin"
)

var (
	scheme = runtime.NewScheme()

	schedConfigFile string
)

func init() {
	v1.AddToScheme(scheme)
}

func NewSchedulerPluginCmd() *cobra.Command {
	// Use is kube-scheduler
	cmd := app.NewSchedulerCommand(
		app.WithPlugin(PluginName, New),
	)

	// add extra flags
	cmd.Flags().StringVar(&schedConfigFile, "custom-sched-cfg-file", "", "path of custom scheduling config file")

	return cmd
}

func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	klog.Infof("loading custom sched config from file %s", schedConfigFile)
	cfg, err := config.Load(schedConfigFile)
	if err != nil {
		return nil, err
	}

	kubeConfig := h.KubeConfig()
	kubeConfig.UserAgent = util.KubeConfigUserAgent
	kubeConfig.QPS = 1000
	kubeConfig.Burst = 1000
	kubeConfig.ContentType = "application/json"

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	state := state.NewState()

	nodeInformerLister := h.SharedInformerFactory().Core().V1().Nodes()
	scInformerLister := h.SharedInformerFactory().Storage().V1().StorageClasses()
	pvcInformerLister := h.SharedInformerFactory().Core().V1().PersistentVolumeClaims()

	pvcInformer := pvcInformerLister.Informer()
	pvcInformer.AddEventHandler(handler.NewPVCEventHandler(state, kubeClient))

	mgr := controllers.NewAndInitControllerManager(controllers.NewManagerRequest{
		MetricsAddr:     ":9090",
		HealthProbeAddr: ":9080",
		WebhookPort:     9443,
		State:           state,

		KubeConfig: kubeConfig,
		KubeCli:    kubeClient,
		Scheme:     scheme,
	})

	ctx := ctrl.SetupSignalHandler()
	go func() {
		klog.Info("starting controller manager")
		if err = mgr.Start(ctx); err != nil {
			klog.Fatal(err)
		}
		klog.Info("controller manager quit")
	}()

	// wait for cache
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		return nil, errors.New("cache cannot sync")
	}

	klog.Info("All cache are synced")

	schedPlugin := &AntstorSchdulerPlugin{
		handle:             h,
		State:              state,
		NodeLister:         nodeInformerLister.Lister(),
		PVCLister:          pvcInformerLister.Lister(),
		StorageClassLister: scInformerLister.Lister(),
		KCli:               kubeClient,
		CustomConfig:       cfg,
	}

	return schedPlugin, nil
}
