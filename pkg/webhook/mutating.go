package webhook

import (
	"context"

	kwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

func podMutatingHandler(ctx context.Context, req kwebhook.AdmissionRequest) kwebhook.AdmissionResponse {

	return kwebhook.Patched("some changes",
		kwebhook.JSONPatchOp{Operation: "add", Path: "/metadata/annotations/access", Value: "granted"},
		kwebhook.JSONPatchOp{Operation: "add", Path: "/metadata/annotations/reason", Value: "not so secret"},
	)
}
