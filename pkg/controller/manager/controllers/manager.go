package controllers

import (
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	rt "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "code.alipay.com/dbplatform/node-disk-controller/pkg/api/volume.antstor.alipay.com/v1"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/kubeutil"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/config"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/handler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	sched "code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/scheduler"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/generated/clientset/versioned"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util/misc"
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

	poolReconciler := reconciler.PlugableReconciler{
		Client:   mgr.GetClient(),
		Plugable: plugin.NewPluginList(),

		Log:     rt.Log.WithName("Controller:StoragePool"),
		KubeCli: kubeClient,
		State:   stateObj,

		Concurrency: 4,
		MainHandler: &reconciler.StoragePoolReconcileHandler{
			Client:   mgr.GetClient(),
			Cfg:      req.ControllerConfig,
			State:    stateObj,
			PoolUtil: poolUtil,
			KubeCli:  kubeClient,
		},
		ForType: &v1.StoragePool{},
		Watches: []reconciler.WatchObject{
			{
				Source: &source.Kind{Type: &corev1.Node{}},
				EventHandler: &handler.NodeEventHandler{
					Cfg: req.ControllerConfig,
				},
			},
		},
	}
	if err = poolReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller StoragePoolReconciler")
		os.Exit(1)
	}

	// setup AntstorVolumeReconciler
	volReconciler := reconciler.PlugableReconciler{
		Client:   mgr.GetClient(),
		Plugable: plugin.NewPluginList(),

		Log:     rt.Log.WithName("Controller:AntstorVolume"),
		KubeCli: kubeClient,
		State:   stateObj,

		Concurrency: 1,
		MainHandler: &reconciler.AntstorVolumeReconcileHandler{
			Client:      mgr.GetClient(),
			State:       stateObj,
			AntstoreCli: antstorCli,
			Scheduler:   scheduler,
		},
		ForType: &v1.AntstorVolume{},
		Watches: []reconciler.WatchObject{
			{
				Source: &source.Kind{Type: &v1.AntstorVolume{}},
				EventHandler: &handler.VolumeEventHandler{
					State: stateObj,
				}},
		},
		Indexes: []reconciler.IndexObject{
			{
				Obj:   &v1.AntstorVolume{},
				Field: v1.IndexKeyUUID,
				ExtractValue: func(rawObj client.Object) []string {
					// grab the volume, extract the uuid
					if vol, ok := rawObj.(*v1.AntstorVolume); ok {
						return []string{vol.Spec.Uuid}
					}
					return nil
				},
			},
			{
				Obj:   &v1.AntstorVolume{},
				Field: v1.IndexKeyTargetNodeID,
				ExtractValue: func(rawObj client.Object) []string {
					// grab the volume, extract the targetNodeId
					if vol, ok := rawObj.(*v1.AntstorVolume); ok {
						return []string{vol.Spec.TargetNodeId}
					}
					return nil
				},
			},
		},
	}
	if err = volReconciler.SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller VolumeReconciler")
		os.Exit(1)
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
		ForType: &v1.AntstorVolumeGroup{},
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
		ForType: &v1.AntstorDataControl{},
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
