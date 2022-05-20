package index

import (
	"context"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TemplateChannelField IndexField = ".spec.channel"

type TemplateChannelIndex struct{}

func NewTemplateChannelIndex() Indexed {
	return &TemplateChannelIndex{}
}

func (idx *TemplateChannelIndex) IndexWith(ctx context.Context, indexer client.FieldIndexer) {
	indexer.IndexField(ctx, &v1alpha1.ModuleTemplate{}, string(TemplateChannelField), func(o client.Object) []string {
		return []string{string((o.(*v1alpha1.ModuleTemplate)).Spec.Channel)}
	})
}
