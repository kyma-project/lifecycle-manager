package index

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const TemplateChannelField Field = "spec.channel"

type TemplateChannelIndex struct{}

func TemplateChannel() *TemplateChannelIndex {
	return &TemplateChannelIndex{}
}

func (idx *TemplateChannelIndex) With(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &v1beta2.ModuleTemplate{}, string(TemplateChannelField),
		func(o client.Object) []string {
			return []string{(o.(*v1beta2.ModuleTemplate)).Spec.Channel}
		})
}
