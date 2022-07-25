package release

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/index"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type TemplateInChannel struct {
	ModuleTemplate *operatorv1alpha1.ModuleTemplate
	Channel        *operatorv1alpha1.Channel
	Outdated       bool
}

type TemplatesInChannels map[string]*TemplateInChannel

func GetTemplates(ctx context.Context, c client.Reader, kyma *operatorv1alpha1.Kyma) (TemplatesInChannels, error) {
	logger := log.FromContext(ctx)
	templates := make(TemplatesInChannels)

	for _, module := range kyma.Spec.Modules {
		template, err := LookupTemplate(c, module, kyma.Spec.Channel, kyma.Spec.Profile).WithContext(ctx)
		if err != nil {
			return nil, err
		}

		templates[module.Name] = template
	}

	CheckForOutdatedTemplates(logger, kyma, templates)

	return templates, nil
}

func CheckForOutdatedTemplates(logger logr.Logger, k *operatorv1alpha1.Kyma, templates TemplatesInChannels) {
	// in the case that the kyma spec did not change, we only have to verify
	// that all desired templates are still referenced in the latest spec generation
	for componentName, lookupResult := range templates {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && lookupResult != nil {
				if lookupResult.ModuleTemplate.GetGeneration() != condition.TemplateInfo.Generation ||
					lookupResult.ModuleTemplate.Spec.Channel != condition.TemplateInfo.Channel {
					logger.Info("detected outdated template",
						"condition", condition.Reason,
						"template", lookupResult.ModuleTemplate.Name,
						"newTemplateGeneration", lookupResult.ModuleTemplate.GetGeneration(),
						"previousTemplateGeneration", condition.TemplateInfo.Generation,
						"newTemplateChannel", lookupResult.ModuleTemplate.Spec.Channel,
						"previousTemplateChannel", condition.TemplateInfo.Channel,
					)

					lookupResult.Outdated = true
				}
			}
		}
	}
}

type Lookup interface {
	WithContext(ctx context.Context) (*TemplateInChannel, error)
}

func LookupTemplate(client client.Reader, module operatorv1alpha1.Module,
	defaultChannel operatorv1alpha1.Channel, profile operatorv1alpha1.Profile,
) *ChannelTemplateLookup {
	return &ChannelTemplateLookup{
		reader:         client,
		module:         module,
		defaultChannel: defaultChannel,
		profile:        profile,
	}
}

type ChannelTemplateLookup struct {
	reader         client.Reader
	module         operatorv1alpha1.Module
	defaultChannel operatorv1alpha1.Channel
	profile        operatorv1alpha1.Profile
}

func (c *ChannelTemplateLookup) WithContext(ctx context.Context) (*TemplateInChannel, error) {
	templateList := &operatorv1alpha1.ModuleTemplateList{}

	desiredChannel := c.getDesiredChannel()

	selector := operatorv1alpha1.GetMatchingLabelsForModule(&c.module, c.profile)

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
			return nil, fmt.Errorf("no module template found for module: %s", c.module.Name)
		}
	}

	template := templateList.Items[0]
	actualChannel := template.Spec.Channel

	// if the found configMap has no defaultChannel assigned to it set a sensible log output
	if actualChannel == "" {
		return nil, fmt.Errorf(
			"no defaultChannel found on template for module: %s, specifying no defaultChannel is not allowed",
			c.module.Name)
	}

	const logLevel = 3
	if actualChannel != c.defaultChannel {
		log.FromContext(ctx).V(logLevel).Info(fmt.Sprintf("using %s (instead of %s) for module %s",
			actualChannel, c.defaultChannel, c.module.Name))
	} else {
		log.FromContext(ctx).V(logLevel).Info(fmt.Sprintf("using %s for module %s",
			actualChannel, c.module.Name))
	}

	return &TemplateInChannel{
		ModuleTemplate: &template,
		Channel:        &actualChannel,
		Outdated:       false,
	}, nil
}

func (c *ChannelTemplateLookup) getDesiredChannel() operatorv1alpha1.Channel {
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

	return fmt.Errorf("more than one module template found for module: %s, candidates: %v",
		component.Name, candidates)
}
