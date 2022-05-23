package index

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Indexed interface {
	IndexWith(ctx context.Context, indexer client.FieldIndexer)
}

type IndexField string

func (f IndexField) WithValue(v string) client.MatchingFields {
	return client.MatchingFields{
		string(f): v,
	}
}
