package moduletemplateinfolookup

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
)

// ByVersionStrategy looks up the module template for a given version-based installation.
type ByVersionStrategy struct {
	client client.Reader
}

func NewByVersionStrategy(client client.Reader) ByVersionStrategy {
	return ByVersionStrategy{client: client}
}

func (ByVersionStrategy) IsResponsible(
	moduleInfo *templatelookup.ModuleInfo,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) bool {
	if moduleReleaseMeta != nil {
		return false
	}

	if !moduleInfo.IsInstalledByVersion() {
		return false
	}

	return true
}

func (s ByVersionStrategy) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	_ *v1beta2.Kyma,
	_ *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	info := templatelookup.ModuleTemplateInfo{
		DesiredChannel: string(shared.NoneChannel),
	}
	template, err := s.filterTemplatesByVersion(ctx, moduleInfo.Name, moduleInfo.Version)
	if err != nil {
		info.Err = err
		return info
	}

	info.ModuleTemplate = template
	return info
}

func (s ByVersionStrategy) filterTemplatesByVersion(ctx context.Context, name, version string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := s.client.List(ctx, templateList)
	if err != nil {
		return nil, fmt.Errorf("failed to list module templates on lookup: %w", err)
	}

	var filteredTemplates []*v1beta2.ModuleTemplate
	for _, template := range templateList.Items {
		if TemplateNameMatch(&template,
			name) && shared.NoneChannel.Equals(template.Spec.Channel) && template.Spec.Version == version {
			filteredTemplates = append(filteredTemplates, &template)
			continue
		}
	}
	if len(filteredTemplates) > 1 {
		return nil, newMoreThanOneTemplateCandidateErr(name, templateList.Items)
	}
	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: for module %s in version %s",
			common.ErrNoTemplatesInListResult, name, version)
	}
	return filteredTemplates[0], nil
}
