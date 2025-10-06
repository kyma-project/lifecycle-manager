package log

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap" //nolint:importas // a one-time reference for the package
)

func ConfigLogger(level int8, syncer zapcore.WriteSyncer) logr.Logger {
	if level > 0 {
		level = -level
	}
	// The following settings is based on kyma community Improvement of log messages usability
	//nolint:revive // https://github.com/kyma-project/community/blob/main/concepts/observability-consistent-logging/improvement-of-log-messages-usability.md#log-structure
	atomicLevel := zap.NewAtomicLevelAt(zapcore.Level(level))
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "date"
	encoderConfig.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	core := zapcore.NewCore(
		&k8szap.KubeAwareEncoder{Encoder: encoder}, syncer, atomicLevel,
	)
	zapLog := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	logger := zapr.NewLogger(zapLog.With(zap.Namespace("context")))
	return logger
}
