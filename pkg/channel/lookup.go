package channel

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var (
	ErrTemplateNotIdentified            = errors.New("no unique template could be identified")
	ErrNotDefaultChannelAllowed         = errors.New("specifying no default channel is not allowed")
	ErrNoTemplatesInListResult          = errors.New("no templates were found")
	ErrInvalidRemoteModuleConfiguration = errors.New("invalid remote module template configuration")
	ErrTemplateNotAllowed               = errors.New("module template not allowed")
	ErrTemplateUpdateNotAllowed         = errors.New("module template update not allowed")
)

type ModuleTemplateTO struct {
	*v1beta2.ModuleTemplate
	Err            error
	DesiredChannel string
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplateTO

func GetTemplates(
	ctx context.Context, kymaClient client.Reader, kyma *v1beta2.Kyma,
) ModuleTemplatesByModuleName {
	logger := ctrlLog.FromContext(ctx)
	templates := make(ModuleTemplatesByModuleName)

	for _, module := range kyma.Spec.Modules {
		var template ModuleTemplateTO

		switch {
		case module.RemoteModuleTemplateRef == "":
			template = NewTemplateLookup(kymaClient, module, kyma.Spec.Channel).WithContext(ctx)
		case kyma.SyncEnabled():
			runtimeClient := remote.SyncContextFromContext(ctx).RuntimeClient
			originalModuleName := module.Name
			module.Name = module.RemoteModuleTemplateRef // To search template with the Remote Ref
			template = NewTemplateLookup(runtimeClient, module, kyma.Spec.Channel).WithContext(ctx)
			module.Name = originalModuleName
		default:
			template.Err = fmt.Errorf("enable sync to use a remote module template for %s: %w", module.Name,
				ErrInvalidRemoteModuleConfiguration)
		}

		templates[module.Name] = &template
	}

	DetermineTemplatesVisibility(kyma, templates)
	CheckValidTemplatesUpdate(logger, kyma, templates)

	return templates
}

func DetermineTemplatesVisibility(kyma *v1beta2.Kyma, templates ModuleTemplatesByModuleName) {
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
}

func CheckValidTemplatesUpdate(logger logr.Logger, kyma *v1beta2.Kyma, templates ModuleTemplatesByModuleName) {
	// in the case that the kyma spec did not change, we only have to verify
	// that all desired templates are still referenced in the latest spec generation
	for moduleName, moduleTemplate := range templates {
		moduleTemplate := moduleTemplate
		for i := range kyma.Status.Modules {
			moduleStatus := &kyma.Status.Modules[i]
			if moduleMatch(moduleStatus, moduleName) && moduleTemplate.ModuleTemplate != nil {
				CheckValidTemplateUpdate(logger, moduleTemplate, moduleStatus)
			}
		}
		templates[moduleName] = moduleTemplate
	}
}

func moduleMatch(moduleStatus *v1beta2.ModuleStatus, moduleName string) bool {
	return moduleStatus.FQDN == moduleName || moduleStatus.Name == moduleName
}

// CheckValidTemplateUpdate verifies if the given ModuleTemplate is valid for update and sets their IsValidUpdate Flag
// based on provided Modules, provided by the Cluster as a status of the last known module state.
// It does this by looking into selected key properties:
// 1. If the generation of ModuleTemplate changes, it means the spec is outdated
// 2. If the channel of ModuleTemplate changes, it means the kyma has an old reference to a previous channel.
//
//nolint:funlen
func CheckValidTemplateUpdate(
	logger logr.Logger, moduleTemplate *ModuleTemplateTO, moduleStatus *v1beta2.ModuleStatus,
) {
	if moduleStatus.Template == nil {
		return
	}

	checkLog := logger.WithValues("module", moduleStatus.FQDN,
		"template", moduleTemplate.Name,
		"newTemplateGeneration", moduleTemplate.GetGeneration(),
		"previousTemplateGeneration", moduleStatus.Template.Generation,
		"newTemplateChannel", moduleTemplate.Spec.Channel,
		"previousTemplateChannel", moduleStatus.Channel,
	)

	if moduleTemplate.Spec.Channel != moduleStatus.Channel {
		checkLog.Info("outdated ModuleTemplate: channel skew")

		descriptor, err := moduleTemplate.Spec.GetDescriptor()
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
			msg := "ignore channel skew, as a higher version of the module was previously installed"
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

type Lookup interface {
	WithContext(ctx context.Context) (*ModuleTemplateTO, error)
}

func NewTemplateLookup(client client.Reader, module v1beta2.Module,
	defaultChannel string,
) *TemplateLookup {
	return &TemplateLookup{
		reader:         client,
		module:         module,
		defaultChannel: defaultChannel,
	}
}

type TemplateLookup struct {
	reader         client.Reader
	module         v1beta2.Module
	defaultChannel string
}

func (c *TemplateLookup) WithContext(ctx context.Context) ModuleTemplateTO {
	desiredChannel := c.getDesiredChannel()

	template, err := c.getTemplate(ctx, desiredChannel)
	if err != nil {
		return ModuleTemplateTO{
			ModuleTemplate: nil,
			DesiredChannel: desiredChannel,
			Err:            err,
		}
	}

	actualChannel := template.Spec.Channel

	// ModuleTemplates without a Channel are not allowed
	if actualChannel == "" {
		return ModuleTemplateTO{
			ModuleTemplate: nil,
			DesiredChannel: desiredChannel,
			Err: fmt.Errorf(
				"no channel found on template for module: %s: %w",
				c.module.Name, ErrNotDefaultChannelAllowed,
			),
		}
	}

	logger := ctrlLog.FromContext(ctx)
	if actualChannel != c.defaultChannel {
		logger.V(log.DebugLevel).Info(
			fmt.Sprintf(
				"using %s (instead of %s) for module %s",
				actualChannel, c.defaultChannel, c.module.Name,
			),
		)
	} else {
		logger.V(log.DebugLevel).Info(
			fmt.Sprintf(
				"using %s for module %s",
				actualChannel, c.module.Name,
			),
		)
	}

	return ModuleTemplateTO{
		ModuleTemplate: template,
		DesiredChannel: desiredChannel,
		Err:            nil,
	}
}

func (c *TemplateLookup) getDesiredChannel() string {
	var desiredChannel string

	switch {
	case c.module.Channel != "":
		desiredChannel = c.module.Channel
	case c.defaultChannel != "":
		desiredChannel = c.defaultChannel
	default:
		desiredChannel = v1beta2.DefaultChannel
	}

	return desiredChannel
}

func (c *TemplateLookup) getTemplate(ctx context.Context, desiredChannel string) (
	*v1beta2.ModuleTemplate, error,
) {
	templateList := &v1beta2.ModuleTemplateList{}
	err := c.reader.List(ctx, templateList)
	if err != nil {
		return nil, err
	}

	moduleIdentifier := c.module.Name
	var filteredTemplates []v1beta2.ModuleTemplate
	for _, template := range templateList.Items {
		if template.Labels[v1beta2.ModuleName] == moduleIdentifier && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
		if template.ObjectMeta.Name == moduleIdentifier && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
		descriptor, err := template.Spec.GetDescriptor()
		if err != nil {
			return nil, fmt.Errorf("invalid ModuleTemplate descriptor: %w", err)
		}
		if descriptor.Name == moduleIdentifier && template.Spec.Channel == desiredChannel {
			filteredTemplates = append(filteredTemplates, template)
			continue
		}
	}

	if len(filteredTemplates) > 1 {
		return nil, NewMoreThanOneTemplateCandidateErr(c.module, templateList.Items)
	}
	if len(filteredTemplates) == 0 {
		return nil, fmt.Errorf("%w: in channel %s for module %s",
			ErrNoTemplatesInListResult, desiredChannel, moduleIdentifier)
	}
	return &filteredTemplates[0], nil
}

func NewMoreThanOneTemplateCandidateErr(component v1beta2.Module,
	candidateTemplates []v1beta2.ModuleTemplate,
) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}

	return fmt.Errorf("%w: more than one module template found for module: %s, candidates: %v",
		ErrTemplateNotIdentified, component.Name, candidates)
}
