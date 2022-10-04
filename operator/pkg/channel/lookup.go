package channel

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/index"
)

var (
	ErrTemplateNotIdentified    = errors.New("no unique template could be identified")
	ErrNotDefaultChannelAllowed = errors.New("specifying no default channel is not allowed")
)

type ModuleTemplate struct {
	*operatorv1alpha1.ModuleTemplate
	Outdated bool
}

type ModuleTemplatesByModuleName map[string]*ModuleTemplate

func GetTemplates(ctx context.Context, c client.Reader, kyma *operatorv1alpha1.Kyma) (ModuleTemplatesByModuleName, error) {
	logger := log.FromContext(ctx)
	templates := make(ModuleTemplatesByModuleName)

	for _, module := range kyma.Spec.Modules {
		template, err := NewTemplateLookup(c, module, kyma.Spec.Channel).WithContext(ctx)
		if err != nil {
			return nil, err
		}

		templates[module.Name] = template
	}

	CheckForOutdatedTemplates(logger, kyma, templates)

	return templates, nil
}

func CheckForOutdatedTemplates(logger logr.Logger, k *operatorv1alpha1.Kyma, templates ModuleTemplatesByModuleName) {
	// in the case that the kyma spec did not change, we only have to verify
	// that all desired templates are still referenced in the latest spec generation
	for moduleName, moduleTemplate := range templates {
		for i := range k.Status.ModuleStatus {
			moduleInfo := &k.Status.ModuleStatus[i]
			if moduleInfo.ModuleName == moduleName && moduleTemplate != nil {
				CheckForOutdatedTemplate(logger, moduleTemplate, moduleInfo)
			}
		}
	}
}

// CheckForOutdatedTemplate verifies if the given ModuleTemplate is outdated and sets their Outdated Flag based on
// provided ModuleStatus, provided by the Cluster as a status of the last known module state.
// It does this by looking into selected key properties:
// 1. If the generation of ModuleTemplate changes, it means the spec is outdated
// 2. If the channel of ModuleTemplate changes, it means the kyma has an old reference to a previous channel.
func CheckForOutdatedTemplate(
	logger logr.Logger, moduleTemplate *ModuleTemplate, moduleStatus *operatorv1alpha1.ModuleStatus,
) {
	checkLog := logger.WithValues("module", moduleStatus.ModuleName,
		"template", moduleTemplate.Name,
		"newTemplateGeneration", moduleTemplate.GetGeneration(),
		"previousTemplateGeneration", moduleStatus.TemplateInfo.Generation,
		"newTemplateChannel", moduleTemplate.Spec.Channel,
		"previousTemplateChannel", moduleStatus.TemplateInfo.Channel,
	)

	if moduleTemplate.GetGeneration() != moduleStatus.TemplateInfo.Generation {
		checkLog.Info("outdated ModuleTemplate: generation skew")
		moduleTemplate.Outdated = true
		return
	}

	if moduleTemplate.Spec.Channel != moduleStatus.TemplateInfo.Channel {
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

		versionInStatus, err := semver.NewVersion(moduleStatus.TemplateInfo.Version)
		if err != nil {
			checkLog.Error(err, "could not handle channel skew as ModuleStatus contains invalid version")
			return
		}

		checkLog = checkLog.WithValues(
			"previousVersion", versionInTemplate.String(),
			"newVersion", versionInStatus.String(),
		)

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

func NewTemplateLookup(client client.Reader, module operatorv1alpha1.Module,
	defaultChannel operatorv1alpha1.Channel,
) *TemplateLookup {
	return &TemplateLookup{
		reader:         client,
		module:         module,
		defaultChannel: defaultChannel,
	}
}

type TemplateLookup struct {
	reader         client.Reader
	module         operatorv1alpha1.Module
	defaultChannel operatorv1alpha1.Channel
}

func (c *TemplateLookup) WithContext(ctx context.Context) (*ModuleTemplate, error) {
	templateList := &operatorv1alpha1.ModuleTemplateList{}

	desiredChannel := c.getDesiredChannel()

	selector := operatorv1alpha1.GetMatchingLabelsForModule(&c.module)

	if err := c.reader.List(ctx, templateList,
		selector,
		index.TemplateChannelField.WithValue(string(desiredChannel)),
	); err != nil {
		return nil, err
	}

	if len(templateList.Items) > 1 {
		return nil, NewMoreThanOneTemplateCandidateErr(c.module, templateList.Items)
	}

	// if the desiredChannel cannot be found, use the next best available
	if len(templateList.Items) == 0 {
		if err := c.reader.List(ctx, templateList,
			selector,
		); err != nil {
			return nil, err
		}

		if len(templateList.Items) > 1 {
			return nil, NewMoreThanOneTemplateCandidateErr(c.module, templateList.Items)
		}

		if len(templateList.Items) == 0 {
			return nil, fmt.Errorf("%w: no module template found for module: %s", ErrTemplateNotIdentified, c.module.Name)
		}
	}

	template := templateList.Items[0]
	actualChannel := template.Spec.Channel

	// ModuleTemplates without a Channel are not allowed
	if actualChannel == "" {
		return nil, fmt.Errorf(
			"no default channel found on template for module: %s: %w",
			c.module.Name, ErrNotDefaultChannelAllowed)
	}

	const logLevel = 3
	if actualChannel != c.defaultChannel {
		log.FromContext(ctx).V(logLevel).Info(fmt.Sprintf("using %s (instead of %s) for module %s",
			actualChannel, c.defaultChannel, c.module.Name))
	} else {
		log.FromContext(ctx).V(logLevel).Info(fmt.Sprintf("using %s for module %s",
			actualChannel, c.module.Name))
	}

	return &ModuleTemplate{
		ModuleTemplate: &template,
		Outdated:       false,
	}, nil
}

func (c *TemplateLookup) getDesiredChannel() operatorv1alpha1.Channel {
	var desiredChannel operatorv1alpha1.Channel

	switch {
	case c.module.Channel != "":
		desiredChannel = c.module.Channel
	case c.defaultChannel != "":
		desiredChannel = c.defaultChannel
	default:
		desiredChannel = operatorv1alpha1.DefaultChannel
	}

	return desiredChannel
}

func NewMoreThanOneTemplateCandidateErr(component operatorv1alpha1.Module,
	candidateTemplates []operatorv1alpha1.ModuleTemplate,
) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}

	return fmt.Errorf("%w: more than one module template found for module: %s, candidates: %v",
		ErrTemplateNotIdentified, component.Name, candidates)
}
