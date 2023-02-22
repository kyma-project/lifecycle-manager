package index

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TemplateFQDNField Field = "spec.descriptor.component.name"

type TemplateFQDNIndex struct{}

func TemplateFQDN() *TemplateFQDNIndex {
	return &TemplateFQDNIndex{}
}

func (idx *TemplateFQDNIndex) With(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(
		ctx, &v1beta1.ModuleTemplate{}, string(TemplateFQDNField),
		func(o client.Object) []string {
			descriptor, err := o.(*v1beta1.ModuleTemplate).Spec.GetDescriptor()
			if err != nil {
				return nil
			}
			return []string{descriptor.GetName()}
		},
	)
}
