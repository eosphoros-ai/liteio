package misc

import (
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func ZapTimeEncoder() func(o *zap.Options) {
	return func(o *zap.Options) {
		o.TimeEncoder = zapcore.RFC3339TimeEncoder
	}
}
