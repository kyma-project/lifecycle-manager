package index

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TemplateChannelField Field = "spec.channel"

type TemplateChannelIndex struct{}

func TemplateChannel() *TemplateChannelIndex {
	return &TemplateChannelIndex{}
}

func (idx *TemplateChannelIndex) With(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &v1beta1.ModuleTemplate{}, string(TemplateChannelField),
		func(o client.Object) []string {
			return []string{(o.(*v1beta1.ModuleTemplate)).Spec.Channel}
		})
}
