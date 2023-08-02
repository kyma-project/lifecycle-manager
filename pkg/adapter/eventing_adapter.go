package adapter

import (
	"context"

	"k8s.io/client-go/tools/record"
)

type key string

const contextKey key = "EventRecorder"

type EventingAdapter func(eventType, reason, message string)

func RecorderFromContext(ctx context.Context) record.EventRecorder {
	rec, ok := ctx.Value(contextKey).(record.EventRecorder)
	if !ok {
		return nil
	}
	return rec
}

func ContextWithRecorder(ctx context.Context, recorder record.EventRecorder) context.Context {
	return context.WithValue(ctx, contextKey, recorder)
}
