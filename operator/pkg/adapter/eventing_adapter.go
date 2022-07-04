package adapter

import (
	"context"

	"k8s.io/client-go/tools/record"
)

type key string

const ContextKey key = "EventRecorder"

type EventingAdapter func(eventtype, reason, message string)

func RecorderFromContext(ctx context.Context) record.EventRecorder { //nolint:ireturn
	return ctx.Value(ContextKey).(record.EventRecorder)
}

func ContextWithRecorder(ctx context.Context, recorder record.EventRecorder) context.Context {
	return context.WithValue(ctx, ContextKey, recorder)
}
