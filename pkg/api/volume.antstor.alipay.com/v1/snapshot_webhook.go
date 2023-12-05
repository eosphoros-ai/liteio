package v1

import (
	"code.alipay.com/dbplatform/node-disk-controller/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var snaplog = logf.Log.WithName("snapshot-webhook")

func (r *AntstorSnapshot) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-volume-antstor-alipay-com-v1-antstorsnapshot,mutating=true,failurePolicy=fail,sideEffects=None,groups=volume.antstor.alipay.com,resources=antstorsnapshots,verbs=create;update,versions=v1,name=antstorsnapshot-defaulter-webhook,admissionReviewVersions=v1

var _ webhook.Defaulter = &AntstorSnapshot{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *AntstorSnapshot) Default() {
	if remainder := r.Spec.Size % util.FourMiB; remainder > 0 {
		r.Spec.Size = (r.Spec.Size / util.FourMiB) * util.FourMiB
		snaplog.Info("defaulter", "name", r.Name, "set Size=", r.Spec.Size)
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-volume-antstor-alipay-com-v1-antstorsnapshot,mutating=false,failurePolicy=fail,sideEffects=None,groups=volume.antstor.alipay.com,resources=antstorsnapshots,verbs=create;update,versions=v1,name=antstorsnapshot-validate-webhook,admissionReviewVersions=v1

var _ webhook.Validator = &AntstorSnapshot{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AntstorSnapshot) ValidateCreate() error {
	snaplog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AntstorSnapshot) ValidateUpdate(old runtime.Object) error {
	snaplog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AntstorSnapshot) ValidateDelete() error {
	snaplog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
