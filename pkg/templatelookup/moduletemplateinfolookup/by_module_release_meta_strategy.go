package moduletemplateinfolookup

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

// ByModuleReleaseMetaStrategy looks up the module template via the module release meta.
// It only supports channel-based installation.
type ByModuleReleaseMetaStrategy struct {
	client client.Reader
}

func NewByModuleReleaseMetaStrategy(client client.Reader) ByModuleReleaseMetaStrategy {
	return ByModuleReleaseMetaStrategy{client: client}
}

func (ByModuleReleaseMetaStrategy) IsResponsible(
	_ *templatelookup.ModuleInfo,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) bool {
	return moduleReleaseMeta != nil
}

func (s ByModuleReleaseMetaStrategy) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	moduleTemplateInfo := templatelookup.ModuleTemplateInfo{}
	moduleTemplateInfo.DesiredChannel = getDesiredChannel(moduleInfo.Channel, kyma.Spec.Channel)

	var desiredModuleVersion string
	var err error
	if moduleReleaseMeta.Spec.Mandatory != nil {
		desiredModuleVersion, err = templatelookup.GetMandatoryVersionForModule(moduleReleaseMeta)
	} else {
		desiredModuleVersion, err = templatelookup.GetChannelVersionForModule(moduleReleaseMeta,
			moduleTemplateInfo.DesiredChannel)
	}
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	template, err := getTemplateByVersion(ctx,
		s.client,
		moduleInfo.Name,
		desiredModuleVersion,
		kyma.Namespace)
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	moduleTemplateInfo.ModuleTemplate = template
	return moduleTemplateInfo
}
