package moduletemplateinfolookup

import (
	"context"
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

var ErrModuleReleaseMetaRequired = errors.New("module release meta is required")

// Lookup handles module template lookup using the ByModuleReleaseMetaStrategy logic.
// It implements the ModuleTemplateInfoLookupStrategy interface from templatelookup.
type Lookup struct {
	client client.Reader
}

func NewLookup(client client.Reader) Lookup {
	return Lookup{client: client}
}

func (l Lookup) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	// Only handle cases where moduleReleaseMeta is provided
	if moduleReleaseMeta == nil {
		return templatelookup.ModuleTemplateInfo{
			Err: ErrModuleReleaseMetaRequired,
		}
	}

	moduleTemplateInfo := templatelookup.ModuleTemplateInfo{}
	moduleTemplateInfo.DesiredChannel = getDesiredChannel(moduleInfo.Channel, kyma.Spec.Channel)

	var resolvedModuleVersion string
	var err error
	if moduleReleaseMeta.Spec.Mandatory != nil {
		resolvedModuleVersion, err = templatelookup.GetMandatoryVersionForModule(moduleReleaseMeta)
	} else {
		resolvedModuleVersion, err = templatelookup.GetChannelVersionForModule(moduleReleaseMeta,
			moduleTemplateInfo.DesiredChannel)
	}
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	if ocmId, err := ocmidentity.NewComponentId(
		moduleReleaseMeta.Spec.OcmComponentName, resolvedModuleVersion); err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	} else {
		moduleTemplateInfo.ComponentId = ocmId
	}

	template, err := getTemplateByVersion(ctx,
		l.client,
		moduleInfo.Name,
		resolvedModuleVersion,
		kyma.Namespace)
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	moduleTemplateInfo.ModuleTemplate = template
	return moduleTemplateInfo
}
