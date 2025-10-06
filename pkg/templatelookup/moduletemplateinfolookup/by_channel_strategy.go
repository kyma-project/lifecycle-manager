package moduletemplateinfolookup

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
)

var ErrNotDefaultChannelAllowed = errors.New("specifying no default channel is not allowed")

// ByChannelStrategy looks up the module template for a given channel-based installation.
type ByChannelStrategy struct {
	client client.Reader
}

func NewByChannelStrategy(client client.Reader) ByChannelStrategy {
	return ByChannelStrategy{client: client}
}

func (ByChannelStrategy) IsResponsible(
	moduleInfo *templatelookup.ModuleInfo,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) bool {
	if moduleReleaseMeta != nil {
		return false
	}

	if moduleInfo.IsInstalledByVersion() {
		return false
	}

	return true
}

func (s ByChannelStrategy) Lookup(ctx context.Context,
	moduleInfo *templatelookup.ModuleInfo,
	kyma *v1beta2.Kyma,
	_ *v1beta2.ModuleReleaseMeta,
) templatelookup.ModuleTemplateInfo {
	desiredChannel := getDesiredChannel(moduleInfo.Channel, kyma.Spec.Channel)
	info := templatelookup.ModuleTemplateInfo{
		DesiredChannel: desiredChannel,
	}

	template, err := s.filterTemplatesByChannel(ctx, moduleInfo.Name, desiredChannel)
	if err != nil {
		info.Err = err
		return info
	}

	actualChannel := template.Spec.Channel
	if actualChannel == "" {
		info.Err = fmt.Errorf(
			"no channel found on template for module: %s: %w",
			moduleInfo.Name, ErrNotDefaultChannelAllowed,
		)
		return info
	}

	logUsedChannel(ctx, moduleInfo.Name, actualChannel, kyma.Spec.Channel)
	info.ModuleTemplate = template
	return info
}

func (s ByChannelStrategy) filterTemplatesByChannel(ctx context.Context, name, desiredChannel string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := s.client.List(ctx, templateList)
	if err != nil {
		return nil, fmt.Errorf("failed to list module templates on lookup: %w", err)
	}

	var filteredTemplates []*v1beta2.ModuleTemplate
	for _, template := range templateList.Items {
		if TemplateNameMatch(&template, name) && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, &template)
			continue
		}
	}

	if len(filteredTemplates) > 1 {
		return nil, newMoreThanOneTemplateCandidateErr(name, templateList.Items)
	}

	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: for module %s in channel %s ",
			common.ErrNoTemplatesInListResult, name, desiredChannel)
	}

	return filteredTemplates[0], nil
}

func getDesiredChannel(moduleChannel, globalChannel string) string {
	var desiredChannel string

	switch {
	case moduleChannel != "":
		desiredChannel = moduleChannel
	case globalChannel != "":
		desiredChannel = globalChannel
	default:
		desiredChannel = v1beta2.DefaultChannel
	}

	return desiredChannel
}

func logUsedChannel(ctx context.Context, name string, actualChannel string, defaultChannel string) {
	logger := logf.FromContext(ctx)
	if actualChannel != defaultChannel {
		logger.V(log.DebugLevel).Info(
			fmt.Sprintf(
				"using %s (instead of %s) for module %s",
				actualChannel, defaultChannel, name,
			),
		)
	} else {
		logger.V(log.DebugLevel).Info(
			fmt.Sprintf(
				"using %s for module %s",
				actualChannel, name,
			),
		)
	}
}
