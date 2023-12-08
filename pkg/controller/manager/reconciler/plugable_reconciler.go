package reconciler

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/reconciler/plugin"
	"code.alipay.com/dbplatform/node-disk-controller/pkg/controller/manager/state"
)

type PlugableReconcilerIface interface {
	plugin.Plugable
	reconcile.Reconciler
}

type ReconcileHandler interface {
	ResourceName() string
	GetObject(plugin.RequestContent) (runtime.Object, error)
	HandleReconcile(*plugin.Context) plugin.Result
	HandleDeletion(*plugin.Context) plugin.Result
}

type SetupWithManagerIface interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type PlugableReconciler struct {
	client.Client
	plugin.Plugable

	KubeCli kubernetes.Interface
	State   state.StateIface
	Log     logr.Logger

	Concurrency int
	WatchType   client.Object
	MainHandler ReconcileHandler
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlugableReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if setup, ok := r.MainHandler.(SetupWithManagerIface); ok {
		return setup.SetupWithManager(mgr)
	}

	if r.Concurrency <= 0 {
		r.Concurrency = 1
	}
	if r.WatchType == nil {
		return fmt.Errorf("WatchType is nil")
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.Concurrency,
		}).
		For(r.WatchType).
		Complete(r)
}

func (r *PlugableReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		resourceID = req.NamespacedName.String()
		log        = r.Log.WithValues(r.MainHandler.ResourceName(), resourceID)
		err        error
		result     plugin.Result
		obj        runtime.Object
		metaObj    metav1.Object
	)

	if obj, err = r.MainHandler.GetObject(plugin.RequestContent{
		Ctx:     ctx,
		Request: req,
	}); err != nil {
		// When user deleted a volume, a request will be recieved.
		// However the volume does not exists. Therefore the code goes to here
		log.Error(err, "unable to fetch Object")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		if errors.IsNotFound(err) {
			// remove SP from State
			log.Info("cannot find Object in apiserver")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// get metadata
	if metaObj, err = meta.Accessor(obj); err != nil {
		log.Error(err, "cannot access object meta")
		return ctrl.Result{}, err
	}

	// create Context
	var pCtx = &plugin.Context{
		KubeCli: r.KubeCli,
		Client:  r.Client,
		State:   r.State,
		Log:     log,
		ReqCtx: plugin.RequestContent{
			Ctx:     ctx,
			Request: req,
			Object:  obj,
		},
	}

	// not handle delete request
	if metaObj.GetDeletionTimestamp() != nil {
		for _, plugin := range r.Plugable.Plugins() {
			plugin.HandleDeletion(pCtx)
		}
		result = r.MainHandler.HandleDeletion(pCtx)
		return result.Result, result.Error
	}

	// run plugins
	for _, plugin := range r.Plugable.Plugins() {
		result = plugin.Reconcile(pCtx)
		if result.NeedBreak() {
			return result.Result, result.Error
		}
	}

	// main reconciling
	result = r.MainHandler.HandleReconcile(pCtx)
	return result.Result, result.Error
}
