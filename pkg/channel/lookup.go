package channel

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1beta1 "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/index"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var (
	ErrTemplateNotIdentified    = errors.New("no unique template could be identified")
	ErrNotDefaultChannelAllowed = errors.New("specifying no default channel is not allowed")
	ErrNoTemplatesInListResult  = errors.New("no templates were found during listing")
)

type ModuleTemplate struct {
	*operatorv1beta1.ModuleTemplate
	Outdated bool
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplate

func GetTemplates(
	ctx context.Context, client client.Reader, kyma *operatorv1beta1.Kyma,
) (ModuleTemplatesByModuleName, error) {
	logger := ctrlLog.FromContext(ctx)
	templates := make(ModuleTemplatesByModuleName)

	for _, module := range kyma.Spec.Modules {
		template, err := NewTemplateLookup(client, module, kyma.Spec.Channel).WithContext(ctx)
		if err != nil {
			return nil, err
		}

		templates[module.Name] = template
	}

	CheckForOutdatedTemplates(logger, kyma, templates)

	return templates, nil
}

func CheckForOutdatedTemplates(logger logr.Logger, k *operatorv1beta1.Kyma, templates ModuleTemplatesByModuleName) {
	// in the case that the kyma spec did not change, we only have to verify
	// that all desired templates are still referenced in the latest spec generation
	for moduleName, moduleTemplate := range templates {
		for i := range k.Status.Modules {
			moduleStatus := &k.Status.Modules[i]
			if moduleStatus.FQDN == moduleName && moduleTemplate != nil {
				CheckForOutdatedTemplate(logger, moduleTemplate, moduleStatus)
			}
		}
	}
}

// CheckForOutdatedTemplate verifies if the given ModuleTemplate is outdated and sets their Outdated Flag based on
// provided Modules, provided by the Cluster as a status of the last known module state.
// It does this by looking into selected key properties:
// 1. If the generation of ModuleTemplate changes, it means the spec is outdated
// 2. If the channel of ModuleTemplate changes, it means the kyma has an old reference to a previous channel.
func CheckForOutdatedTemplate(
	logger logr.Logger, moduleTemplate *ModuleTemplate, moduleStatus *operatorv1beta1.ModuleStatus,
) {
	checkLog := logger.WithValues("module", moduleStatus.FQDN,
		"template", moduleTemplate.Name,
		"newTemplateGeneration", moduleTemplate.GetGeneration(),
		"previousTemplateGeneration", moduleStatus.Template.Generation,
		"newTemplateChannel", moduleTemplate.Spec.Channel,
		"previousTemplateChannel", moduleStatus.Channel,
	)

	// generation skews always have to be handled. We are not in need of checking downgrades here,
	// since these are catched by our validating webhook. We do not support downgrades of Versions
	// in ModuleTemplates, meaning the only way the generation can be changed is by changing the target
	// channel (valid change) or a version increase
	if moduleTemplate.GetGeneration() != moduleStatus.Template.Generation {
		checkLog.Info("outdated ModuleTemplate: generation skew")
		moduleTemplate.Outdated = true
		return
	}

	if moduleTemplate.Spec.Channel != moduleStatus.Channel {
		checkLog.Info("outdated ModuleTemplate: channel skew")

		descriptor, err := moduleTemplate.Spec.GetDescriptor()
		if err != nil {
			checkLog.Error(err, "could not handle channel skew as descriptor from template cannot be fetched")
			return
		}

		versionInTemplate, err := semver.NewVersion(descriptor.Version)
		if err != nil {
			checkLog.Error(err, "could not handle channel skew as descriptor from template contains invalid version")
			return
		}

		versionInStatus, err := semver.NewVersion(moduleStatus.Version)
		if err != nil {
			checkLog.Error(err, "could not handle channel skew as Modules contains invalid version")
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
		if versionInStatus.GreaterThan(versionInTemplate) {
			checkLog.Info("ignore channel skew, as a higher version of the module was previously installed")
			return
		}

		moduleTemplate.Outdated = true
	}
}

type Lookup interface {
	WithContext(ctx context.Context) (*ModuleTemplate, error)
}

func NewTemplateLookup(client client.Reader, module operatorv1beta1.Module,
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
	module         operatorv1beta1.Module
	defaultChannel string
}

func (c *TemplateLookup) WithContext(ctx context.Context) (*ModuleTemplate, error) {
	desiredChannel := c.getDesiredChannel()

	template, err := c.getTemplate(ctx, desiredChannel)
	if err != nil {
		return nil, err
	}

	actualChannel := template.Spec.Channel

	// ModuleTemplates without a Channel are not allowed
	if actualChannel == "" {
		return nil, fmt.Errorf(
			"no channel found on template for module: %s: %w",
			c.module.Name, ErrNotDefaultChannelAllowed,
		)
	}

	logger := ctrlLog.FromContext(ctx)
	if actualChannel != c.defaultChannel {
		logger.Info(
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

	return &ModuleTemplate{
		ModuleTemplate: template,
		Outdated:       false,
	}, nil
}

func (c *TemplateLookup) getTemplate(
	ctx context.Context, desiredChannel string,
) (*operatorv1beta1.ModuleTemplate, error) {
	lookupVariants := []client.ListOption{
		// first try to find a template with "operator.kyma-project.io/module-name" == module.Name
		operatorv1beta1.ModuleTemplatesByLabel(&c.module),
		// then try to find a template with FQDN (".spec.descriptor.component.name") == module.Name
		index.TemplateFQDNField.WithValue(c.module.Name),
		// then try to find a template with "metadata.name" == module.Name
		index.TemplateNameField.WithValue(c.module.Name),
	}
	var template *operatorv1beta1.ModuleTemplate
	for _, variant := range lookupVariants {
		var err error
		template, err = c.getModuleTemplateFromDesiredChannel(ctx, desiredChannel, variant)
		if err != nil && !errors.Is(err, ErrNoTemplatesInListResult) {
			return nil, err
		}
		if template != nil {
			return template, nil
		}
	}
	return nil, fmt.Errorf(
		"%w: no module template found for module: %s, attempted to lookup via %v", ErrTemplateNotIdentified, c.module.Name,
		lookupVariants,
	)
}

func (c *TemplateLookup) getModuleTemplateFromDesiredChannel(
	ctx context.Context, desiredChannel string, option client.ListOption,
) (*operatorv1beta1.ModuleTemplate, error) {
	templateList := &operatorv1beta1.ModuleTemplateList{}

	var err error
	switch option.(type) {
	case client.MatchingFields:
		templateListPreChannelFilter := &operatorv1beta1.ModuleTemplateList{}
		err = c.reader.List(ctx, templateListPreChannelFilter, option)
		if err != nil {
			return nil, err
		}
		for _, template := range templateListPreChannelFilter.Items {
			if template.Spec.Channel == desiredChannel {
				templateList.Items = append(templateList.Items, template)
			}
		}
	default:
		err = c.reader.List(
			ctx, templateList, option, index.TemplateChannelField.WithValue(desiredChannel),
		)
	}
	if err != nil {
		return nil, err
	}
	if len(templateList.Items) > 1 {
		return nil, NewMoreThanOneTemplateCandidateErr(c.module, templateList.Items, option)
	}
	if len(templateList.Items) == 0 {
		return nil, fmt.Errorf("no templates found with %s in channel %s: %w", option, desiredChannel,
			ErrNoTemplatesInListResult)
	}
	return &templateList.Items[0], nil
}

func (c *TemplateLookup) getDesiredChannel() string {
	var desiredChannel string

	switch {
	case c.module.Channel != "":
		desiredChannel = c.module.Channel
	case c.defaultChannel != "":
		desiredChannel = c.defaultChannel
	default:
		desiredChannel = operatorv1beta1.DefaultChannel
	}

	return desiredChannel
}

func NewMoreThanOneTemplateCandidateErr(component operatorv1beta1.Module,
	candidateTemplates []operatorv1beta1.ModuleTemplate, option client.ListOption,
) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}

	return fmt.Errorf("%w: more than one module template found with %v for module: %s, candidates: %v",
		ErrTemplateNotIdentified, option, component.Name, candidates)
}
