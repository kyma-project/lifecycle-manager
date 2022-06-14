package index

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Indexed interface {
	With(ctx context.Context, indexer client.FieldIndexer) error
}

type Field string

func (f Field) WithValue(v string) client.MatchingFields {
	return client.MatchingFields{
		string(f): v,
	}
}
