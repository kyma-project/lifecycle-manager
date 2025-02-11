package skrresources

import (
	"context"
)

type StringCollector struct {
	entries []interface{}
}

func NewStringCollector() *StringCollector {
	return &StringCollector{}
}

func (c *StringCollector) Collect(ctx context.Context, data string) {
	c.entries = append(c.entries, data)
}

func (c *StringCollector) Emit(ctx context.Context) {
	// Emit collected data to some backing store
}
