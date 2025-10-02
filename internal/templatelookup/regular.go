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
	"github.com/kyma-project/lifecycle-manager/internal/templatelookup/common"
)

var (
	ErrTemplateNotAllowed              = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed        = errors.New("module template update not allowed")
	ErrWaitingForNextMaintenanceWindow = errors.New(
		"waiting for next maintenance window to update module version",
	)
	ErrFailedToDetermineIfMaintenanceWindowIsActive = errors.New("failed to determine if maintenance window is active")
	ErrModuleReleaseMetaNotFound                    = errors.New("ModuleReleaseMeta not found for module")
)

type MaintenanceWindow interface {
	IsRequired(moduleTemplate *v1beta2.ModuleTemplate, kyma *v1beta2.Kyma) bool
	IsActive(kyma *v1beta2.Kyma) (bool, error)
}

type ModuleTemplateInfo struct {
	*v1beta2.ModuleTemplate

	Err            error
	DesiredChannel string
}

type TemplateLookup struct {
	client.Reader

	descriptorProvider *provider.CachedDescriptorProvider
	maintenanceWindow  MaintenanceWindow
}

func NewTemplateLookup(reader client.Reader,
	descriptorProvider *provider.CachedDescriptorProvider,
	maintenanceWindow MaintenanceWindow,
) *TemplateLookup {
	return &TemplateLookup{
		Reader:             reader,
		descriptorProvider: descriptorProvider,
		maintenanceWindow:  maintenanceWindow,
	}
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplateInfo

// LookupModuleTemplate looks up the module template via the module release meta.
// In production, moduleReleaseMeta is guaranteed to exist for valid modules.
func LookupModuleTemplate(ctx context.Context,
	clnt client.Reader,
	moduleInfo *ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) ModuleTemplateInfo {
	moduleTemplateInfo := ModuleTemplateInfo{}
	moduleTemplateInfo.DesiredChannel = getDesiredChannel(moduleInfo.Channel, kyma.Spec.Channel)

	var desiredModuleVersion string
	var err error
	if moduleReleaseMeta.Spec.Mandatory != nil {
		desiredModuleVersion, err = GetMandatoryVersionForModule(moduleReleaseMeta)
	} else {
		desiredModuleVersion, err = GetChannelVersionForModule(moduleReleaseMeta,
			moduleTemplateInfo.DesiredChannel)
	}
	if err != nil {
		moduleTemplateInfo.Err = err
		return moduleTemplateInfo
	}

	template, err := getTemplateByVersion(ctx,
		clnt,
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

// getDesiredChannel determines the desired channel based on module-specific channel or global channel.
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

func getTemplateByVersion(ctx context.Context,
	clnt client.Reader,
	moduleName,
	moduleVersion,
	namespace string,
) (*v1beta2.ModuleTemplate, error) {
	moduleTemplate := &v1beta2.ModuleTemplate{}

	moduleTemplateName := fmt.Sprintf("%s-%s", moduleName, moduleVersion)
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      moduleTemplateName,
		Namespace: namespace,
	}, moduleTemplate); err != nil {
		return nil, fmt.Errorf("failed to get module template: %w", err)
	}

	return moduleTemplate, nil
}

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

		// If ModuleReleaseMeta doesn't exist, we can't proceed with template lookup
		if moduleReleaseMeta == nil {
			templates[moduleInfo.Name] = &ModuleTemplateInfo{
				Err: fmt.Errorf("%w: %s", ErrModuleReleaseMetaNotFound, moduleInfo.Name),
			}
			continue
		}

		templateInfo := t.lookupModuleTemplateWithMaintenanceWindow(ctx,
			&moduleInfo,
			kyma,
			moduleReleaseMeta)

		templateInfo = ValidateTemplateMode(templateInfo, kyma)
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

// lookupModuleTemplateWithMaintenanceWindow performs the core lookup and applies maintenance window logic.
func (t *TemplateLookup) lookupModuleTemplateWithMaintenanceWindow(ctx context.Context,
	moduleInfo *ModuleInfo,
	kyma *v1beta2.Kyma,
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta,
) ModuleTemplateInfo {
	// First perform the standard lookup
	moduleTemplateInfo := LookupModuleTemplate(ctx, t.Reader, moduleInfo, kyma, moduleReleaseMeta)

	// If lookup failed or no maintenance window configured, return as-is
	if moduleTemplateInfo.ModuleTemplate == nil || moduleTemplateInfo.Err != nil || t.maintenanceWindow == nil {
		return moduleTemplateInfo
	}

	// Check if maintenance window is required for this module template
	if !t.maintenanceWindow.IsRequired(moduleTemplateInfo.ModuleTemplate, kyma) {
		return moduleTemplateInfo
	}

	// Check if maintenance window is currently active
	active, err := t.maintenanceWindow.IsActive(kyma)
	if err != nil {
		moduleTemplateInfo.Err = fmt.Errorf("%w: %w", ErrFailedToDetermineIfMaintenanceWindowIsActive, err)
		moduleTemplateInfo.ModuleTemplate = nil
		return moduleTemplateInfo
	}

	if !active {
		moduleTemplateInfo.Err = ErrWaitingForNextMaintenanceWindow
		moduleTemplateInfo.ModuleTemplate = nil
		return moduleTemplateInfo
	}

	return moduleTemplateInfo
}

func ValidateTemplateMode(template ModuleTemplateInfo,
	kyma *v1beta2.Kyma,
) ModuleTemplateInfo {
	if template.Err != nil {
		return template
	}

	return validateTemplateMode(template, kyma)
}

func validateTemplateMode(template ModuleTemplateInfo, kyma *v1beta2.Kyma) ModuleTemplateInfo {
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

// TemplateNameMatch checks if a module template matches the given name.
func TemplateNameMatch(template *v1beta2.ModuleTemplate, name string) bool {
	return template.Spec.ModuleName == name
}
