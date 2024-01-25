package templatelookup

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

var (
	ErrTemplateNotIdentified            = errors.New("no unique template could be identified")
	ErrNotDefaultChannelAllowed         = errors.New("specifying no default channel is not allowed")
	ErrNoTemplatesInListResult          = errors.New("no templates were found")
	ErrTemplateMarkedAsMandatory        = errors.New("template marked as mandatory")
	ErrInvalidRemoteModuleConfiguration = errors.New("invalid remote module template configuration")
	ErrTemplateNotAllowed               = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed         = errors.New("module template update not allowed")
)

type ModuleTemplateInfo struct {
	*v1beta2.ModuleTemplate
	Err            error
	DesiredChannel string
}

func NewTemplateLookup(reader client.Reader, descriptorProvider *provider.CachedDescriptorProvider, syncEnabled bool) *TemplateLookup {
	return &TemplateLookup{
		Reader:             reader,
		descriptorProvider: descriptorProvider,
		syncEnabled:        syncEnabled,
	}
}

type TemplateLookup struct {
	client.Reader
	descriptorProvider *provider.CachedDescriptorProvider
	syncEnabled        bool
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplateInfo

func (t *TemplateLookup) GetRegularTemplates(ctx context.Context, kyma *v1beta2.Kyma) ModuleTemplatesByModuleName {
	templates := make(ModuleTemplatesByModuleName)
	for _, module := range kyma.GetAvailableModules() {
		var template ModuleTemplateInfo
		_, found := templates[module.Name]
		if found {
			continue
		}
		switch {
		case module.RemoteModuleTemplateRef == "":
			template = t.getAndValidate(ctx, module.Name, module.Channel, kyma.Spec.Channel)
			if template.Err != nil {
				break
			}
			if err := t.descriptorProvider.Add(template.ModuleTemplate); err != nil {
				template.Err = fmt.Errorf("failed to get descriptor: %w", err)
			}

		case t.syncEnabled:
			originalModuleName := module.Name
			module.Name = module.RemoteModuleTemplateRef // To search template with the Remote Ref
			template = t.remoteGetAndValidate(ctx, module.Name, module.Channel, kyma.Spec.Channel)
			module.Name = originalModuleName
		default:
			template.Err = fmt.Errorf("enable sync to use a remote module template for %s: %w", module.Name,
				ErrInvalidRemoteModuleConfiguration)
		}

		templates[module.Name] = &template
	}

	for moduleName, moduleTemplate := range templates {
		if moduleTemplate.Err != nil {
			continue
		}

		if moduleTemplate.IsInternal() && !kyma.IsInternal() {
			moduleTemplate.Err = fmt.Errorf("%w: internal module", ErrTemplateNotAllowed)
			templates[moduleName] = moduleTemplate
		}
		if moduleTemplate.IsBeta() && !kyma.IsBeta() {
			moduleTemplate.Err = fmt.Errorf("%w: beta module", ErrTemplateNotAllowed)
			templates[moduleName] = moduleTemplate
		}
	}

	for moduleName, moduleTemplate := range templates {
		template := moduleTemplate
		for i := range kyma.Status.Modules {
			moduleStatus := &kyma.Status.Modules[i]
			if moduleMatch(moduleStatus, moduleName) && template.ModuleTemplate != nil {
				t.checkValidTemplateUpdate(ctx, template, moduleStatus)
			}
		}
		templates[moduleName] = template
	}

	return templates
}

func (t *TemplateLookup) getAndValidate(ctx context.Context, name, channel, defaultChannel string) ModuleTemplateInfo {
	desiredChannel := getDesiredChannel(channel, defaultChannel)
	info := ModuleTemplateInfo{
		DesiredChannel: desiredChannel,
	}

	template, err := t.getTemplate(ctx, t, name, desiredChannel)
	if err != nil {
		info.Err = err
		return info
	}

	actualChannel := template.Spec.Channel
	if actualChannel == "" {
		info.Err = fmt.Errorf(
			"no channel found on template for module: %s: %w",
			name, ErrNotDefaultChannelAllowed,
		)
		return info
	}

	logUsedChannel(ctx, name, actualChannel, defaultChannel)
	info.ModuleTemplate = template
	return info
}

func (t *TemplateLookup) remoteGetAndValidate(ctx context.Context, name, channel, defaultChannel string) ModuleTemplateInfo {
	desiredChannel := getDesiredChannel(channel, defaultChannel)
	info := ModuleTemplateInfo{
		DesiredChannel: desiredChannel,
	}
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		info.Err = fmt.Errorf("failed to get syncContext: %w", err)
		return info
	}
	runtimeClient := syncContext.RuntimeClient

	template, err := t.getTemplate(ctx, runtimeClient, name, desiredChannel)
	if err != nil {
		info.Err = err
		return info
	}

	actualChannel := template.Spec.Channel
	// ModuleTemplates without a Channel are not allowed
	if actualChannel == "" {
		info.Err = fmt.Errorf(
			"no channel found on template for module: %s: %w",
			name, ErrNotDefaultChannelAllowed,
		)
		return info
	}

	logUsedChannel(ctx, name, actualChannel, defaultChannel)

	info.ModuleTemplate = template
	return info
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

func moduleMatch(moduleStatus *v1beta2.ModuleStatus, moduleName string) bool {
	return moduleStatus.FQDN == moduleName || moduleStatus.Name == moduleName
}

// checkValidTemplateUpdate verifies if the given ModuleTemplate is valid for update and sets their IsValidUpdate Flag
// based on provided Modules, provided by the Cluster as a status of the last known module state.
// It does this by looking into selected key properties:
// 1. If the generation of ModuleTemplate changes, it means the spec is outdated
// 2. If the channel of ModuleTemplate changes, it means the kyma has an old reference to a previous channel.
func (t *TemplateLookup) checkValidTemplateUpdate(
	ctx context.Context, moduleTemplate *ModuleTemplateInfo, moduleStatus *v1beta2.ModuleStatus,
) {
	if moduleStatus.Template == nil {
		return
	}
	logger := logf.FromContext(ctx)
	checkLog := logger.WithValues("module", moduleStatus.FQDN,
		"template", moduleTemplate.Name,
		"newTemplateGeneration", moduleTemplate.GetGeneration(),
		"previousTemplateGeneration", moduleStatus.Template.Generation,
		"newTemplateChannel", moduleTemplate.Spec.Channel,
		"previousTemplateChannel", moduleStatus.Channel,
	)

	if moduleTemplate.Spec.Channel != moduleStatus.Channel {
		checkLog.Info("outdated ModuleTemplate: channel skew")

		descriptor, err := t.descriptorProvider.GetDescriptor(moduleTemplate.ModuleTemplate)
		if err != nil {
			msg := "could not handle channel skew as descriptor from template cannot be fetched"
			checkLog.Error(err, msg)
			moduleTemplate.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
			return
		}

		versionInTemplate, err := semver.NewVersion(descriptor.Version)
		if err != nil {
			msg := "could not handle channel skew as descriptor from template contains invalid version"
			checkLog.Error(err, msg)
			moduleTemplate.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
			return
		}

		versionInStatus, err := semver.NewVersion(moduleStatus.Version)
		if err != nil {
			msg := "could not handle channel skew as Modules contains invalid version"
			checkLog.Error(err, msg)
			moduleTemplate.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
			return
		}

		checkLog = checkLog.WithValues(
			"previousVersion", versionInTemplate.String(),
			"newVersion", versionInStatus.String(),
		)

		// channel skews have to be handled with more detail. If a channel is changed this means
		// that the downstream kyma might have changed its target channel for the module, meaning
		// the old moduleStatus is reflecting the previous desired state.
		// when increasing channel stability, this means we could potentially have a downgrade
		// of module versions here (fast: v2.0.0 get downgraded to regular: v1.0.0). In this
		// case we want to suspend updating the module until we reach v2.0.0 in regular, since downgrades
		// are not supported. To circumvent this, a module can be uninstalled and then reinstalled in the old channel.
		if !v1beta2.IsValidVersionChange(versionInTemplate, versionInStatus) {
			msg := fmt.Sprintf("ignore channel skew (from %s to %s), "+
				"as a higher version (%s) of the module was previously installed",
				moduleStatus.Channel, moduleTemplate.Spec.Channel, versionInStatus.String())
			checkLog.Info(msg)
			moduleTemplate.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
			return
		}

		return
	}

	// generation skews always have to be handled. We are not in need of checking downgrades here,
	// since these are caught by our validating webhook. We do not support downgrades of Versions
	// in ModuleTemplates, meaning the only way the generation can be changed is by changing the target
	// channel (valid change) or a version increase
	if moduleTemplate.GetGeneration() != moduleStatus.Template.Generation {
		checkLog.Info("outdated ModuleTemplate: generation skew")
		return
	}
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

func (t *TemplateLookup) getTemplate(ctx context.Context, clnt client.Reader, name, desiredChannel string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := clnt.List(ctx, templateList)
	if err != nil {
		return nil, fmt.Errorf("failed to list module templates on lookup: %w", err)
	}

	var filteredTemplates []v1beta2.ModuleTemplate
	for _, template := range templateList.Items {
		if template.Labels[shared.ModuleName] == name && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
		if fmt.Sprintf("%s/%s", template.Namespace, template.Name) == name &&
			template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
		if template.ObjectMeta.Name == name && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
		descriptor, err := t.descriptorProvider.GetDescriptor(&template)
		if err != nil {
			return nil, fmt.Errorf("invalid ModuleTemplate descriptor: %w", err)
		}
		if descriptor.Name == name && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
	}

	if len(filteredTemplates) > 1 {
		return nil, NewMoreThanOneTemplateCandidateErr(name, templateList.Items)
	}
	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: in channel %s for module %s",
			ErrNoTemplatesInListResult, desiredChannel, name)
	}
	if filteredTemplates[0].Spec.Mandatory {
		return nil, fmt.Errorf("%w: in channel %s for module %s",
			ErrTemplateMarkedAsMandatory, desiredChannel, name)
	}
	return &filteredTemplates[0], nil
}

func NewMoreThanOneTemplateCandidateErr(moduleName string,
	candidateTemplates []v1beta2.ModuleTemplate,
) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}

	return fmt.Errorf("%w: more than one module template found for module: %s, candidates: %v",
		ErrTemplateNotIdentified, moduleName, candidates)
}
