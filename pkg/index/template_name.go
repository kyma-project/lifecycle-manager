package index

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

const TemplateNameField Field = "metadata.name"

type TemplateNameIndex struct{}

func TemplateName() *TemplateNameIndex {
	return &TemplateNameIndex{}
}

func (idx *TemplateNameIndex) With(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &v1beta1.ModuleTemplate{}, string(TemplateNameField),
		func(o client.Object) []string {
			template := o.(*v1beta1.ModuleTemplate)
			return []string{
				template.GetName(),
				fmt.Sprintf("%s/%s", template.GetNamespace(), template.GetName()),
			}
		})
}
