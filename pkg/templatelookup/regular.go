package templatelookup

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
)

var (
	ErrTemplateNotAllowed       = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed = errors.New("module template update not allowed")
)

type ModuleTemplateInfo struct {
	*v1beta2.ModuleTemplate

	Err            error
	DesiredChannel string
}

type ModuleTemplateInfoLookupStrategy interface {
	Lookup(ctx context.Context,
		moduleInfo *ModuleInfo,
		kyma *v1beta2.Kyma,
		moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
	) ModuleTemplateInfo
}

type TemplateLookup struct {
	client.Reader

	descriptorProvider               *provider.CachedDescriptorProvider
	moduleTemplateInfoLookupStrategy ModuleTemplateInfoLookupStrategy
}

func NewTemplateLookup(reader client.Reader,
	descriptorProvider *provider.CachedDescriptorProvider,
	moduleTemplateInfoLookupStrategy ModuleTemplateInfoLookupStrategy,
) *TemplateLookup {
	return &TemplateLookup{
		Reader:                           reader,
		descriptorProvider:               descriptorProvider,
		moduleTemplateInfoLookupStrategy: moduleTemplateInfoLookupStrategy,
	}
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplateInfo

func (t *TemplateLookup) GetRegularTemplates(ctx context.Context, kyma *v1beta2.Kyma) ModuleTemplatesByModuleName {
	templates := make(ModuleTemplatesByModuleName)
	for _, moduleInfo := range FetchModuleInfo(kyma) {
		_, found := templates[moduleInfo.Name]
		if found {
			continue
		}
		if moduleInfo.ValidationError != nil {
			templates[moduleInfo.Name] = &ModuleTemplateInfo{Err: moduleInfo.ValidationError}
			continue
		}

		moduleReleaseMeta, err := GetModuleReleaseMeta(ctx, t, moduleInfo.Name, kyma.Namespace)
		if client.IgnoreNotFound(err) != nil {
			templates[moduleInfo.Name] = &ModuleTemplateInfo{Err: err}
			continue
		}

		templateInfo := t.moduleTemplateInfoLookupStrategy.Lookup(ctx,
			&moduleInfo,
			kyma,
			moduleReleaseMeta)

		templateInfo = ValidateTemplateMode(templateInfo)
		if templateInfo.Err != nil {
			templates[moduleInfo.Name] = &templateInfo
			continue
		}
		if err := t.descriptorProvider.Add(templateInfo.ModuleTemplate); err != nil {
			templateInfo.Err = fmt.Errorf("failed to get descriptor: %w", err)
			templates[moduleInfo.Name] = &templateInfo
			continue
		}
		for i := range kyma.Status.Modules {
			moduleStatus := &kyma.Status.Modules[i]
			if moduleMatch(moduleStatus, moduleInfo.Name) {
				descriptor, err := t.descriptorProvider.GetDescriptor(templateInfo.ModuleTemplate)
				if err != nil {
					msg := "could not handle channel skew as descriptor from template cannot be fetched"
					templateInfo.Err = fmt.Errorf("%w: %s", ErrTemplateUpdateNotAllowed, msg)
					continue
				}
				markInvalidSkewUpdate(ctx, &templateInfo, moduleStatus, descriptor.Version)
			}
		}
		templates[moduleInfo.Name] = &templateInfo
	}
	return templates
}

func ValidateTemplateMode(template ModuleTemplateInfo) ModuleTemplateInfo {
	if template.Err != nil {
		return template
	}

	return validateTemplateMode(template)
}

func validateTemplateMode(template ModuleTemplateInfo) ModuleTemplateInfo {
	if template.Spec.Mandatory {
		template.Err = fmt.Errorf("%w: for module %s in channel %s ",
			common.ErrNoTemplatesInListResult, template.Name, template.DesiredChannel)
		return template
	}
	return template
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
