package templatelookup

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrTemplateNotIdentified     = errors.New("no unique template could be identified")
	ErrNotDefaultChannelAllowed  = errors.New("specifying no default channel is not allowed")
	ErrNoTemplatesInListResult   = errors.New("no templates were found")
	ErrTemplateMarkedAsMandatory = errors.New("template marked as mandatory")
	ErrTemplateNotAllowed        = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed  = errors.New("module template update not allowed")
)

type ModuleTemplateInfo struct {
	*v1beta2.ModuleTemplate
	Err            error
	DesiredChannel string
}

func NewTemplateLookup(reader client.Reader, descriptorProvider *provider.CachedDescriptorProvider) *TemplateLookup {
	return &TemplateLookup{
		Reader:             reader,
		descriptorProvider: descriptorProvider,
	}
}

type TemplateLookup struct {
	client.Reader
	descriptorProvider *provider.CachedDescriptorProvider
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplateInfo

func (t *TemplateLookup) GetRegularTemplates(ctx context.Context, kyma *v1beta2.Kyma) ModuleTemplatesByModuleName {
	templates := make(ModuleTemplatesByModuleName)
	for _, module := range FindAvailableModules(kyma) {
		_, found := templates[module.Name]
		if found {
			continue
		}
		if module.ValidationError != nil {
			templates[module.Name] = &ModuleTemplateInfo{Err: module.ValidationError}
			continue
		}

		templateInfo := t.PopulateModuleTemplateInfo(ctx, module, kyma.Namespace, kyma.Spec.Channel)
		templateInfo = t.ValidateTemplateMode(ctx, templateInfo, kyma)
		if templateInfo.Err != nil {
			templates[module.Name] = &templateInfo
			continue
		}
		if err := t.descriptorProvider.Add(templateInfo.ModuleTemplate); err != nil {
			templateInfo.Err = fmt.Errorf("failed to get descriptor: %w", err)
			templates[module.Name] = &templateInfo
			continue
		}
		for i := range kyma.Status.Modules {
			moduleStatus := &kyma.Status.Modules[i]
			if moduleMatch(moduleStatus, module.Name) {
				descriptor, err := t.descriptorProvider.GetDescriptor(templateInfo.ModuleTemplate)
				if err != nil {
					msg := "could not handle channel skew as descriptor from template cannot be fetched"
					templateInfo.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
					continue
				}
				markInvalidSkewUpdate(ctx, &templateInfo, moduleStatus, descriptor.Version)
			}
		}
		templates[module.Name] = &templateInfo
	}
	return templates
}

func (t *TemplateLookup) PopulateModuleTemplateInfo(ctx context.Context,
	module AvailableModule, namespace, kymaChannel string,
) ModuleTemplateInfo {
	moduleReleaseMeta, err := GetModuleReleaseMeta(ctx, t, module.Name, namespace)
	if util.IsNotFound(err) {
		return t.populateModuleTemplateInfoWithoutModuleReleaseMeta(ctx, module, kymaChannel)
	}

	if err != nil {
		return ModuleTemplateInfo{Err: err}
	}

	return t.populateModuleTemplateInfoUsingModuleReleaseMeta(ctx, module, moduleReleaseMeta, kymaChannel, namespace)
}

func (t *TemplateLookup) populateModuleTemplateInfoWithoutModuleReleaseMeta(ctx context.Context,
	module AvailableModule, kymaChannel string,
) ModuleTemplateInfo {
	var templateInfo ModuleTemplateInfo
	if module.IsInstalledByVersion() {
		templateInfo = t.GetAndValidateByVersion(ctx, module.Name, module.Version)
	} else {
		templateInfo = t.GetAndValidateByChannel(ctx, module.Name, module.Channel, kymaChannel)
	}
	return templateInfo
}

func (t *TemplateLookup) populateModuleTemplateInfoUsingModuleReleaseMeta(ctx context.Context,
	module AvailableModule,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta, kymaChannel, namespace string,
) ModuleTemplateInfo {
	var templateInfo ModuleTemplateInfo
	templateInfo.DesiredChannel = getDesiredChannel(module.Channel, kymaChannel)
	desiredModuleVersion, err := GetChannelVersionForModule(moduleReleaseMeta, templateInfo.DesiredChannel)
	if err != nil {
		templateInfo.Err = err
		return templateInfo
	}

	template, err := t.getTemplateByVersion(ctx, module.Name, desiredModuleVersion, namespace)
	if err != nil {
		templateInfo.Err = err
		return templateInfo
	}

	templateInfo.ModuleTemplate = template

	return templateInfo
}

func (t *TemplateLookup) ValidateTemplateMode(ctx context.Context, template ModuleTemplateInfo, kyma *v1beta2.Kyma) ModuleTemplateInfo {
	if template.Err != nil {
		return template
	}

	moduleReleaseMeta, err := GetModuleReleaseMeta(ctx, t, template.Spec.ModuleName, template.Namespace)

	if util.IsNotFound(err) {
		return validateTemplateModeWithoutModuleReleaseMeta(template, kyma)
	}

	return validateTemplateModeWithModuleReleaseMeta(template, kyma, moduleReleaseMeta)
}

func validateTemplateModeWithoutModuleReleaseMeta(template ModuleTemplateInfo, kyma *v1beta2.Kyma) ModuleTemplateInfo {
	if template.IsInternal() && !kyma.IsInternal() {
		template.Err = fmt.Errorf("%w: internal module", ErrTemplateNotAllowed)
		return template
	}
	if template.IsBeta() && !kyma.IsBeta() {
		template.Err = fmt.Errorf("%w: beta module", ErrTemplateNotAllowed)
		return template
	}
	return template
}

func validateTemplateModeWithModuleReleaseMeta(template ModuleTemplateInfo, kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) ModuleTemplateInfo {
	if !remote.IsAllowedModuleReleaseMeta(*moduleReleaseMeta, kyma) {
		template.Err = fmt.Errorf("%w: module is beta or internal", ErrTemplateNotAllowed)
	}

	return template
}

func (t *TemplateLookup) getTemplateByVersion(ctx context.Context,
	moduleName, moduleVersion, namespace string,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplate := &v1beta2.ModuleTemplate{}

	moduleTemplateName := fmt.Sprintf("%s-%s", moduleName, moduleVersion)
	if err := t.Get(ctx, client.ObjectKey{
		Name:      moduleTemplateName,
		Namespace: namespace,
	}, moduleTemplate); err != nil {
		return nil, fmt.Errorf("failed to get module template: %w", err)
	}

	return moduleTemplate, nil
}

func (t *TemplateLookup) GetAndValidateByChannel(ctx context.Context,
	name, channel, defaultChannel string,
) ModuleTemplateInfo {
	desiredChannel := getDesiredChannel(channel, defaultChannel)
	info := ModuleTemplateInfo{
		DesiredChannel: desiredChannel,
	}

	template, err := t.filterTemplatesByChannel(ctx, name, desiredChannel)
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

func (t *TemplateLookup) GetAndValidateByVersion(ctx context.Context, name, version string) ModuleTemplateInfo {
	info := ModuleTemplateInfo{
		DesiredChannel: string(shared.NoneChannel),
	}
	template, err := t.filterTemplatesByVersion(ctx, name, version)
	if err != nil {
		info.Err = err
		return info
	}

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
	return moduleStatus.Name == moduleName
}

// markInvalidSkewUpdate verifies if the given ModuleTemplate is invalid for update.
func markInvalidSkewUpdate(ctx context.Context, moduleTemplateInfo *ModuleTemplateInfo,
	moduleStatus *v1beta2.ModuleStatus, templateVersion string,
) {
	if moduleStatus.Template == nil {
		return
	}
	if moduleTemplateInfo == nil || moduleTemplateInfo.Err != nil {
		return
	}

	logger := logf.FromContext(ctx)
	checkLog := logger.WithValues("module", moduleStatus.FQDN,
		"template", moduleTemplateInfo.Name,
		"newTemplateGeneration", moduleTemplateInfo.GetGeneration(),
		"previousTemplateGeneration", moduleStatus.Template.GetGeneration(),
		"newTemplateChannel", moduleTemplateInfo.DesiredChannel,
		"previousTemplateChannel", moduleStatus.Channel,
	)

	checkLog.Info("outdated ModuleTemplate: channel skew")

	versionInTemplate, err := semver.NewVersion(templateVersion)
	if err != nil {
		msg := "could not handle channel skew as descriptor from template contains invalid version"
		checkLog.Error(err, msg)
		moduleTemplateInfo.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
		return
	}

	versionInStatus, err := semver.NewVersion(moduleStatus.Version)
	if err != nil {
		msg := "could not handle channel skew as Modules contains invalid version"
		checkLog.Error(err, msg)
		moduleTemplateInfo.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
		return
	}

	checkLog = checkLog.WithValues(
		"previousVersion", versionInTemplate.String(),
		"newVersion", versionInStatus.String(),
	)

	if !isValidVersionChange(versionInTemplate, versionInStatus) {
		msg := fmt.Sprintf("ignore channel skew (from %s to %s), "+
			"as a higher version (%s) of the module was previously installed",
			moduleStatus.Channel, moduleTemplateInfo.DesiredChannel, versionInStatus.String())
		checkLog.Info(msg)
		moduleTemplateInfo.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
	}
}

func isValidVersionChange(newVersion *semver.Version, oldVersion *semver.Version) bool {
	filteredNewVersion := filterVersion(newVersion)
	filteredOldVersion := filterVersion(oldVersion)
	return !filteredNewVersion.LessThan(filteredOldVersion)
}

func filterVersion(version *semver.Version) *semver.Version {
	filteredVersion, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d",
		version.Major(), version.Minor(), version.Patch()))
	return filteredVersion
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

func (t *TemplateLookup) filterTemplatesByChannel(ctx context.Context, name, desiredChannel string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := t.List(ctx, templateList)
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
		return nil, NewMoreThanOneTemplateCandidateErr(name, templateList.Items)
	}

	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: for module %s in channel %s ",
			ErrNoTemplatesInListResult, name, desiredChannel)
	}

	if filteredTemplates[0].Spec.Mandatory {
		return nil, fmt.Errorf("%w: for module %s in channel %s",
			ErrTemplateMarkedAsMandatory, name, desiredChannel)
	}

	return filteredTemplates[0], nil
}

func (t *TemplateLookup) filterTemplatesByVersion(ctx context.Context, name, version string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := t.List(ctx, templateList)
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
		return nil, NewMoreThanOneTemplateCandidateErr(name, templateList.Items)
	}
	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: for module %s in version %s",
			ErrNoTemplatesInListResult, name, version)
	}
	if filteredTemplates[0].Spec.Mandatory {
		return nil, fmt.Errorf("%w: for module %s in version %s",
			ErrTemplateMarkedAsMandatory, name, version)
	}
	return filteredTemplates[0], nil
}

func TemplateNameMatch(template *v1beta2.ModuleTemplate, name string) bool {
	if len(template.Spec.ModuleName) > 0 {
		return template.Spec.ModuleName == name
	}

	// Drop the legacyCondition once the label 'shared.ModuleName' is removed: https://github.com/kyma-project/lifecycle-manager/issues/1796
	if template.Labels == nil {
		return false
	}
	return template.Labels[shared.ModuleName] == name
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
