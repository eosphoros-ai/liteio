package webhook

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	kwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Run webhook server",
		Long:  `Run webhook server`,
		Run: func(cmd *cobra.Command, args []string) {
			Run()
		},
	}
	return cmd
}

func Run() {
	// Create a manager
	// Note: GetConfigOrDie will os.Exit(1) w/o any message if no kube-config can be found
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})
	if err != nil {
		klog.Fatal(err)
	}

	// Create a webhook server.
	hookServer := &kwebhook.Server{
		Port: 8443,
	}
	if err := mgr.Add(hookServer); err != nil {
		klog.Fatal(err)
	}

	mutatingHook := &kwebhook.Admission{
		Handler: admission.HandlerFunc(podMutatingHandler),
	}

	// Register the webhooks in the server.
	hookServer.Register("/mutating", mutatingHook)
	// hookServer.Register("/validating", validatingHook)

	// Start the server by starting a previously-set-up manager
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		// handle error
		klog.Fatal(err)
	}
}
