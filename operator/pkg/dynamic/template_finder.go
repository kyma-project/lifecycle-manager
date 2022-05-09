package dynamic

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewTemplateFinder(reader client.Reader) *TemplateFinder {
	return &TemplateFinder{Reader: reader}
}

type TemplateFinder struct {
	client.Reader
	component string
	release   string
}

func (t *TemplateFinder) For(component string) *TemplateFinder {
	t.component = component
	return t
}

func (t *TemplateFinder) Release(release string) *TemplateFinder {
	t.release = release
	return t
}

func (t *TemplateFinder) Find(ctx context.Context) (*corev1.ConfigMap, error) {
	if t.component == "" {
		return nil, errors.New("component name has to be supplied to find a template")
	}
	if t.release == "" {
		return nil, errors.New("release has to be supplied to find a template")
	}

	configMapList := &corev1.ConfigMapList{}
	if err := t.List(ctx, configMapList,
		client.MatchingLabels{
			labels.ControllerName: t.component,
			labels.Release:        t.release,
		},
	); err != nil {
		return nil, err
	}

	if len(configMapList.Items) > 1 {
		return nil, fmt.Errorf("more than one config map template found for component: %s(%s)", t.component, t.release)
	}

	if len(configMapList.Items) == 0 {
		return nil, fmt.Errorf("no config map template found for component: %s(%s)", t.component, t.release)
	}

	return &configMapList.Items[0], nil
}
