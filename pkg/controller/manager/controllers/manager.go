package controllers

import (
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	sched "code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type B64EncodedMysqlDSN string

func (m B64EncodedMysqlDSN) DSN() string {
	return misc.B64DecStr(string(m))
}

type NewManagerRequest struct {
	MetricsAddr     string
	HealthProbeAddr string
	SyncDBConnInfo  string
	K8SCluster      string

	// webhook
	EnableWebhook bool
	WebhookPort   int
	// The server key and certificate must be named tls.key and tls.crt
	WebhookTLSDir string

	State      state.StateIface
	KubeConfig *rest.Config
	KubeCli    kubernetes.Interface
	Scheme     *runtime.Scheme

	ControllerConfig config.Config
}

func NewAndInitControllerManager(req NewManagerRequest) manager.Manager {
	var (
		err        error
		mgr        manager.Manager
		kubeCfg    = req.KubeConfig
		kubeClient = req.KubeCli
	)

	rt.SetLogger(zap.New(zap.UseDevMode(true), misc.ZapTimeEncoder()))

	mgr, err = rt.NewManager(kubeCfg, rt.Options{
		Scheme:                 req.Scheme,
		MetricsBindAddress:     req.MetricsAddr,
		HealthProbeBindAddress: req.HealthProbeAddr,
		LeaderElection:         false,
		LeaderElectionID:       "911ffb70.antstor.alipay.com",
		// Port of webhook service
		Port: req.WebhookPort,
		// The server key and certificate must be named tls.key and tls.crt
		CertDir: req.WebhookTLSDir,
	})
	if err != nil {
		klog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	var (
		antstorCli = versioned.NewForConfigOrDie(kubeCfg)
		stateObj   = req.State
		poolUtil   = kubeutil.NewStoragePoolUtil(mgr.GetClient())
		scheduler  = sched.NewScheduler(req.ControllerConfig)
	)

	// setup StoragePoolReconciler
	poolReconciler := &reconciler.StoragePoolReconciler{
		Plugable: plugin.NewPluginList(),
		Client:   mgr.GetClient(),
		Log:      rt.Log.WithName("controllers").WithName("StoragePool"),
		State:    stateObj,
		PoolUtil: poolUtil,
		KubeCli:  kubeClient,
		Lock:     misc.NewResourceLocks(),
	}

	// setup AntstorVolumeReconciler
	volReconciler := &reconciler.AntstorVolumeReconciler{
		Plugable:    plugin.NewPluginList(),
		Client:      mgr.GetClient(),
		Log:         rt.Log.WithName("controllers").WithName("AntstorVolume"),
		State:       stateObj,
		AntstoreCli: antstorCli,
		Scheduler:   scheduler,
		// EventRecorder for AntstorVolume
		EventRecorder: mgr.GetEventRecorderFor("AntstorVolume"),
	}

	// setup AntstorVolumeGroupReconciler
	volGroupReconciler := reconciler.PlugableReconciler{
		Client:   mgr.GetClient(),
		Plugable: plugin.NewPluginList(),

		Log:     rt.Log.WithName("Controller:AntstorVolumeGroup"),
		KubeCli: kubeClient,
		State:   stateObj,

		Concurrency: 1,
		MainHandler: &reconciler.AntstorVolumeGroupReconcileHandler{
			Client:    mgr.GetClient(),
			Scheduler: scheduler,
			State:     stateObj,
		},
		WatchType: &v1.AntstorDataControl{},
	}
	if err = volGroupReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller VolumeGroupReconciler")
		os.Exit(1)
	}

	// setup AntstorDataControlReconciler
	dataControlReconciler := reconciler.PlugableReconciler{
		Client:   mgr.GetClient(),
		Plugable: plugin.NewPluginList(),

		Log:     rt.Log.WithName("Controller:AntstorDataControl"),
		KubeCli: kubeClient,
		State:   stateObj,

		Concurrency: 1,
		MainHandler: &reconciler.AntstorDataControlReconcileHandler{
			Client: mgr.GetClient(),
		},
		WatchType: &v1.AntstorDataControl{},
	}
	if err = dataControlReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller AntstorDataControlReconciler")
		os.Exit(1)
	}

	// register plugins
	pluginHandle := &PluginHandle{
		Req:           req,
		Client:        mgr.GetClient(),
		Mgr:           mgr,
		AntstorClient: antstorCli,
	}

	for _, fn := range PoolReconcilerPluginCreaters {
		p, err := fn(pluginHandle)
		if err != nil {
			klog.Fatal(err)
		}
		poolReconciler.RegisterPlugin(p)
	}

	for _, fn := range VolumeReconcilerPluginCreaters {
		p, err := fn(pluginHandle)
		if err != nil {
			klog.Fatal(err)
		}
		volReconciler.RegisterPlugin(p)
	}

	for _, fn := range VolumeGroupReconcilerPluginCreaters {
		p, err := fn(pluginHandle)
		if err != nil {
			klog.Fatal(err)
		}
		volGroupReconciler.RegisterPlugin(p)
	}

	for _, fn := range DataControlReconcilerPluginCreaters {
		p, err := fn(pluginHandle)
		if err != nil {
			klog.Fatal(err)
		}
		dataControlReconciler.RegisterPlugin(p)
	}

	// setup StoragePool/Volume Reconciler
	if err = poolReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller StoragePoolReconciler")
		os.Exit(1)
	}
	if err = volReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller VolumeReconciler")
		os.Exit(1)
	}

	// setup SnapshotReconsiler
	snapshotReconciler := &reconciler.SnapshotReconciler{
		// kube client
		Client: mgr.GetClient(),
		Log:    rt.Log.WithName("controllers").WithName("Snapshot"),
		Scheme: mgr.GetScheme(),
		// EventRecorder for AntstorSnapshot
		EventRecorder: mgr.GetEventRecorderFor("AntstorSnapshot"),
		// clientset
		AntstorClientset: antstorCli,
	}
	if err = snapshotReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create Snapshot controller")
		os.Exit(1)
	}

	migrationReconcile := &reconciler.VolumeMigrationReconciler{
		Client: mgr.GetClient(),
		Log:    rt.Log.WithName("controllers").WithName("Migration"),
	}
	if err = migrationReconcile.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create Snapshot controller")
		os.Exit(1)
	}

	// setup state API service
	klog.Infof("setup state API service on %s, URI /state/storagepool", req.MetricsAddr)
	mgr.AddMetricsExtraHandler("/state/storagepool", state.NewStateHandler(stateObj))

	if req.EnableWebhook {
		klog.Info("setup webhook service for AntstorVolume")
		err = (&v1.AntstorVolume{}).SetupWebhookWithManager(mgr)
		if err != nil {
			klog.Error(err, "unable to setup webhook service for AntstorVolume, use --enableWebhook=false to skip this error")
			os.Exit(1)
		}
	}

	// setup Healthz and Readyz
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	return mgr
}
