package release

import (
	"context"
	"fmt"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConfigMapTemplate interface {
	Lookup(ctx context.Context) (*operatorv1alpha1.ModuleTemplate, error)
}

type TemplatesByName map[string]*operatorv1alpha1.ModuleTemplate

func GetTemplates(c client.Reader, ctx context.Context, k *operatorv1alpha1.Kyma) (TemplatesByName, error) {
	templates := make(map[string]*operatorv1alpha1.ModuleTemplate)
	for _, component := range k.Spec.Components {
		configMap, err := NewChannelConfigMapTemplate(c, component, k.Spec.Channel).Lookup(ctx)
		if err != nil {
			return nil, err
		}
		templates[component.Name] = configMap
	}
	return templates, nil
}

func NewChannelConfigMapTemplate(client client.Reader, component operatorv1alpha1.ComponentType, channel operatorv1alpha1.Channel) ConfigMapTemplate {
	return &channelConfigMapLookup{
		reader:    client,
		component: component,
		channel:   channel,
	}
}

type channelConfigMapLookup struct {
	reader    client.Reader
	component operatorv1alpha1.ComponentType
	channel   operatorv1alpha1.Channel
}

func (c *channelConfigMapLookup) Lookup(ctx context.Context) (*operatorv1alpha1.ModuleTemplate, error) {
	configMapList := &operatorv1alpha1.ModuleTemplateList{}

	var desiredChannel operatorv1alpha1.Channel

	if c.component.Channel != "" {
		// if component channel is set it takes precedence
		desiredChannel = c.component.Channel
	} else if c.channel != "" {
		// else if the global channel is et it takes precedence
		desiredChannel = c.channel
	} else {
		// else use the default channel
		desiredChannel = operatorv1alpha1.DefaultChannel
	}

	if err := c.reader.List(ctx, configMapList,
		client.MatchingLabels{
			labels.ControllerName: c.component.Name,
			labels.Channel:        string(desiredChannel),
		},
	); err != nil {
		return nil, err
	}

	if len(configMapList.Items) > 1 {
		return nil, fmt.Errorf("more than one config map template found for component: %s", c.component.Name)
	}

	// if the desiredChannel cannot be found, use the next best available
	if len(configMapList.Items) == 0 {
		if err := c.reader.List(ctx, configMapList,
			client.MatchingLabels{
				labels.ControllerName: c.component.Name,
			},
		); err != nil {
			return nil, err
		}

		if len(configMapList.Items) > 1 {
			return nil, fmt.Errorf("more than one config map template found for component: %s", c.component.Name)
		}

		if len(configMapList.Items) == 0 {
			return nil, fmt.Errorf("no config map template found for component: %s", c.component.Name)
		}

	}

	actualChannel := operatorv1alpha1.Channel(configMapList.Items[0].GetLabels()[labels.Channel])

	// if the found configMap has no channel assigned to it set a sensible log output
	if actualChannel == "" {
		return nil, fmt.Errorf("no channel found on template for component: %s, specifying no channel is not allowed", c.component.Name)
	}

	if actualChannel != c.channel {
		log.FromContext(ctx).V(0).Info(fmt.Sprintf("using %s (instead of %s) for component %s", actualChannel, c.channel, c.component.Name))
	} else {
		log.FromContext(ctx).V(0).Info(fmt.Sprintf("using %s for component %s", actualChannel, c.component.Name))
	}

	return &configMapList.Items[0], nil
}
