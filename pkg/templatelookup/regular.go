package templatelookup

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	restrictedmodulesvc "github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
)

var (
	ErrTemplateNotAllowed       = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed = errors.New("module template update not allowed")
	ErrNoModuleReleaseMeta      = errors.New("no ModuleReleaseMeta found")
	ErrNoIdentity               = errors.New("component identity is nil")
)

type ModuleTemplateInfo struct {
	*v1beta2.ModuleTemplate

	Err            error
	DesiredChannel string // This is the channel that was requested by the user
	//                       using Kyma 'spec.channel' or configured module channel.

	ComponentId *ocmidentity.ComponentId // Identifies the OCM Component that is
	//                                          represented by this ModuleTemplateInfo.
}

// GetOCMIdentity implements provider.OCMIProvider.
func (m ModuleTemplateInfo) GetOCMIdentity() (*ocmidentity.ComponentId, error) {
	if m.ComponentId == nil {
		return nil, fmt.Errorf("%w for module template %s", ErrNoIdentity, m.Name)
	}
	return m.ComponentId, nil
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
	restrictedModules                []string
}

func NewTemplateLookup(reader client.Reader,
	descriptorProvider *provider.CachedDescriptorProvider,
	moduleTemplateInfoLookupStrategy ModuleTemplateInfoLookupStrategy,
	restrictedModules []string,
) *TemplateLookup {
	return &TemplateLookup{
		Reader:                           reader,
		descriptorProvider:               descriptorProvider,
		moduleTemplateInfoLookupStrategy: moduleTemplateInfoLookupStrategy,
		restrictedModules:                restrictedModules,
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

		if moduleReleaseMeta == nil {
			msg := fmt.Sprintf(" for module %q in namespace %q",
				moduleInfo.Name, kyma.Namespace)
			templates[moduleInfo.Name] = &ModuleTemplateInfo{Err: fmt.Errorf("%w %s", ErrNoModuleReleaseMeta, msg)}
			continue
		}

		templateInfo := t.moduleTemplateInfoLookupStrategy.Lookup(ctx,
			&moduleInfo,
			kyma,
			moduleReleaseMeta)

		templateInfo = t.ValidateTemplateMode(templateInfo, kyma, moduleReleaseMeta)
		if templateInfo.Err != nil {
			templates[moduleInfo.Name] = &templateInfo
			continue
		}

		ocmId, err := ocmidentity.NewComponentId(moduleReleaseMeta.Spec.OcmComponentName, templateInfo.Spec.Version)
		if err != nil {
			templateInfo.Err = fmt.Errorf("failed to create OCM Component Identity: %w", err)
			templates[moduleInfo.Name] = &templateInfo
			continue
		}

		if err := t.descriptorProvider.Add(*ocmId); err != nil {
			templateInfo.Err = fmt.Errorf("failed to get descriptor: %w", err)
			templates[moduleInfo.Name] = &templateInfo
			continue
		}
		for i := range kyma.Status.Modules {
			moduleStatus := &kyma.Status.Modules[i]
			if moduleMatch(moduleStatus, moduleInfo.Name) {
				markInvalidSkewUpdate(ctx, &templateInfo, moduleStatus, ocmId.Version())
			}
		}
		templates[moduleInfo.Name] = &templateInfo
	}
	return templates
}

func (t *TemplateLookup) ValidateTemplateMode(template ModuleTemplateInfo,
	kyma *v1beta2.Kyma,
	mrm *v1beta2.ModuleReleaseMeta,
) ModuleTemplateInfo {
	if template.Err != nil {
		return template
	}

	return t.validateTemplateMode(template, kyma, mrm)
}

func (t *TemplateLookup) validateTemplateMode(template ModuleTemplateInfo,
	kyma *v1beta2.Kyma,
	mrm *v1beta2.ModuleReleaseMeta,
) ModuleTemplateInfo {
	if template.IsInternal() && !kyma.IsInternal() {
		template.Err = fmt.Errorf("%w: internal module", ErrTemplateNotAllowed)
		return template
	}
	if template.IsBeta() && !kyma.IsBeta() {
		template.Err = fmt.Errorf("%w: beta module", ErrTemplateNotAllowed)
		return template
	}
	if template.Spec.Mandatory {
		template.Err = fmt.Errorf("%w: for module %s in channel %s ",
			common.ErrNoTemplatesInListResult, template.Name, template.DesiredChannel)
		return template
	}
	if mrm != nil {
		inRestrictedList := slices.Contains(t.restrictedModules, mrm.Spec.ModuleName)
		hasSelector := mrm.Spec.KymaSelector != nil

		switch {
		case !inRestrictedList && !hasSelector:
			// normal module, allow
		case !inRestrictedList && hasSelector:
			template.Err = fmt.Errorf("%w: module has kymaSelector but is not in restricted modules list",
				ErrTemplateNotAllowed)
			return template
		case inRestrictedList && !hasSelector:
			template.Err = ErrTemplateNotAllowed
			return template
		case inRestrictedList && hasSelector:
			matched, err := restrictedmodulesvc.RestrictedModuleMatch(mrm, kyma)
			if err != nil {
				template.Err = fmt.Errorf("%w: %w", ErrTemplateNotAllowed, err)
				return template
			}
			if !matched {
				template.Err = fmt.Errorf("%w: restricted module not allowed for this Kyma", ErrTemplateNotAllowed)
				return template
			}
		}
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
	ocmComponentName := moduleStatus.OCMComponentName
	if ocmComponentName == "" {
		ocmComponentName = moduleStatus.FQDN //nolint:staticcheck // Fallback for the time of migration
	}
	checkLog := logger.WithValues("module", ocmComponentName,
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
