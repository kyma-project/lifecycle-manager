package index

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TemplateChannelField Field = ".spec.channel"

type TemplateChannelIndex struct{}

func TemplateChannel() *TemplateChannelIndex {
	return &TemplateChannelIndex{}
}

func (idx *TemplateChannelIndex) With(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &v1alpha1.ModuleTemplate{}, string(TemplateChannelField),
		func(o client.Object) []string {
			return []string{string((o.(*v1alpha1.ModuleTemplate)).Spec.Channel)}
		})
}
