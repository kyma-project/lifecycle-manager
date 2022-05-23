package release

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/index"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type LookupIrrecoverableErr error

type TemplateLookupResult struct {
	Template   *operatorv1alpha1.ModuleTemplate
	forChannel *operatorv1alpha1.Channel
}
type Template interface {
	Lookup(ctx context.Context) (*TemplateLookupResult, error)
}

type TemplateLookupResultsByName map[string]*TemplateLookupResult

func GetTemplates(c client.Reader, ctx context.Context, k *operatorv1alpha1.Kyma) (TemplateLookupResultsByName, error) {
	templates := make(TemplateLookupResultsByName)
	for _, component := range k.Spec.Components {
		template, err := NewChannelTemplate(c, component, k.Spec.Channel).Lookup(ctx)
		if err != nil {
			return nil, err
		}
		templates[component.Name] = template
	}
	return templates, nil
}

func NewChannelTemplate(client client.Reader, component operatorv1alpha1.ComponentType, channel operatorv1alpha1.Channel) Template {
	return &channelTemplateLookup{
		reader:    client,
		component: component,
		channel:   channel,
	}
}

type channelTemplateLookup struct {
	reader    client.Reader
	component operatorv1alpha1.ComponentType
	channel   operatorv1alpha1.Channel
}

func (c *channelTemplateLookup) Lookup(ctx context.Context) (*TemplateLookupResult, error) {
	templateList := &operatorv1alpha1.ModuleTemplateList{}

	var desiredChannel operatorv1alpha1.Channel

	if c.component.Channel != "" {
		// if component channel is set it takes precedence
		desiredChannel = c.component.Channel
	} else if c.channel != "" {
		// else if the global channel is set it takes precedence
		desiredChannel = c.channel
	} else {
		// else use the default channel
		desiredChannel = operatorv1alpha1.DefaultChannel
	}

	if err := c.reader.List(ctx, templateList,
		client.MatchingLabels{
			labels.ControllerName: c.component.Name,
		},
		index.TemplateChannelField.WithValue(string(desiredChannel)),
	); err != nil {
		return nil, err
	}

	if len(templateList.Items) > 1 {
		return nil, MoreThanOneTemplateCandidateErr(c.component, templateList.Items)
	}

	// if the desiredChannel cannot be found, use the next best available
	if len(templateList.Items) == 0 {
		if err := c.reader.List(ctx, templateList,
			client.MatchingLabels{
				labels.ControllerName: c.component.Name,
			},
		); err != nil {
			return nil, err
		}

		if len(templateList.Items) > 1 {
			return nil, MoreThanOneTemplateCandidateErr(c.component, templateList.Items)
		}

		if len(templateList.Items) == 0 {
			return nil, fmt.Errorf("no config map template found for component: %s", c.component.Name)
		}

	}

	actualChannel := templateList.Items[0].Spec.Channel

	// if the found configMap has no channel assigned to it set a sensible log output
	if actualChannel == "" {
		return nil, fmt.Errorf("no channel found on template for component: %s, specifying no channel is not allowed", c.component.Name)
	}

	if actualChannel != c.channel {
		log.FromContext(ctx).V(3).Info(fmt.Sprintf("using %s (instead of %s) for component %s", actualChannel, c.channel, c.component.Name))
	} else {
		log.FromContext(ctx).V(3).Info(fmt.Sprintf("using %s for component %s", actualChannel, c.component.Name))
	}

	return &TemplateLookupResult{
		Template:   &templateList.Items[0],
		forChannel: &actualChannel,
	}, nil
}

func MoreThanOneTemplateCandidateErr(component operatorv1alpha1.ComponentType, candidateTemplates []operatorv1alpha1.ModuleTemplate) error {
	candidates := make([]string, len(candidateTemplates))
	for i, candidate := range candidateTemplates {
		candidates[i] = candidate.GetName()
	}
	return fmt.Errorf("more than one config map template found for component: %s, candidates: %v", component.Name, candidates)
}
