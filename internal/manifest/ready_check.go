package manifest

import (
	"context"

	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

func NewResourceReadyCheck() *ResourceReadyCheck {
	return &ResourceReadyCheck{}
}

type ResourceReadyCheck struct{}

type ResourceKind string

const (
	DeploymentKind  ResourceKind = "Deployment"
	StatefulSetKind ResourceKind = "StatefulSet"
)

type Resource struct {
	Kind ResourceKind
	*apiappsv1.Deployment
	*apiappsv1.StatefulSet
}

func (c *ResourceReadyCheck) Run(ctx context.Context,
	clnt declarativev2.Client,
	resources []*resource.Info,
) (shared.State, error) {
	res := findResource(clnt, resources)
	if res == nil {
		return shared.StateReady, nil
	}

	switch res.Kind {
	case StatefulSetKind:
		statefulSetReadyCheck := NewStatefulSetReadyCheck()
		return statefulSetReadyCheck.Run(ctx, clnt, res.StatefulSet)
	case DeploymentKind:
		deploymentReadyCheck := NewDeploymentReadyCheck()
		return deploymentReadyCheck.Run(res.Deployment)
	}

	return shared.StateReady, nil
}

func findResource(clt declarativev2.Client, resources []*resource.Info) *Resource {
	deploy := &apiappsv1.Deployment{}
	statefulSet := &apiappsv1.StatefulSet{}

	for _, res := range resources {
		if err := clt.Scheme().Convert(res.Object, deploy, nil); err == nil {
			return &Resource{
				Kind:       DeploymentKind,
				Deployment: deploy,
			}
		}

		if err := clt.Scheme().Convert(res.Object, statefulSet, nil); err == nil {
			return &Resource{
				Kind:        StatefulSetKind,
				StatefulSet: statefulSet,
			}
		}
	}

	return nil
}
