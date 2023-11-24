package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	// for validatioin
	_ webhook.Defaulter = &AntstorVolume{}
	_ webhook.Validator = &AntstorVolume{}

	// log is for logging in this package.
	avlog = logf.Log.WithName("volume-webhook")
)

func (av *AntstorVolume) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(av).
		Complete()
}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (av *AntstorVolume) Default() {
	avlog.Info("default", "name", av.Name)

	if av.Labels == nil {
		av.Labels = make(map[string]string)
	}

	if _, has := av.Labels[UuidLabelKey]; !has {
		av.Labels[UuidLabelKey] = av.Spec.Uuid
	}

	if av.Status.Status == "" {
		av.Status.Status = VolumeStatusCreating
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (av *AntstorVolume) ValidateCreate() error {
	avlog.Info("validate create", "name", av.Name)

	if av.Spec.Uuid == "" {
		return fmt.Errorf("volume uuid cannot be empty")
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (av *AntstorVolume) ValidateUpdate(old runtime.Object) error {
	avlog.Info("validate update", "name", av.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (av *AntstorVolume) ValidateDelete() error {
	avlog.Info("validate delete", "name", av.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
