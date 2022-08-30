package release

import (
	"context"
	"errors"
	"fmt"

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
		template, err := LookupTemplate(c, module, kyma.Spec.Channel).WithContext(ctx)
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
	for moduleName, lookupResult := range templates {
		for i := range k.Status.ModuleInfos {
			moduleInfo := &k.Status.ModuleInfos[i]
			if moduleInfo.ModuleName == moduleName && lookupResult != nil {
				if lookupResult.ModuleTemplate.GetGeneration() != moduleInfo.TemplateInfo.Generation ||
					lookupResult.ModuleTemplate.Spec.Channel != moduleInfo.TemplateInfo.Channel {
					logger.Info("detected outdated template",
						"module", moduleInfo.ModuleName,
						"template", lookupResult.ModuleTemplate.Name,
						"newTemplateGeneration", lookupResult.ModuleTemplate.GetGeneration(),
						"previousTemplateGeneration", moduleInfo.TemplateInfo.Generation,
						"newTemplateChannel", lookupResult.ModuleTemplate.Spec.Channel,
						"previousTemplateChannel", moduleInfo.TemplateInfo.Channel,
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
	defaultChannel operatorv1alpha1.Channel,
) *ChannelTemplateLookup {
	return &ChannelTemplateLookup{
		reader:         client,
		module:         module,
		defaultChannel: defaultChannel,
	}
}

type ChannelTemplateLookup struct {
	reader         client.Reader
	module         operatorv1alpha1.Module
	defaultChannel operatorv1alpha1.Channel
}

func (c *ChannelTemplateLookup) WithContext(ctx context.Context) (*TemplateInChannel, error) {
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

	// if the found configMap has no defaultChannel assigned to it set a sensible log output
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

	return fmt.Errorf("%w: more than one module template found for module: %s, candidates: %v",
		ErrTemplateNotIdentified, component.Name, candidates)
}
