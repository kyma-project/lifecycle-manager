package statecheck

import (
	"context"

	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

type ManagerStateCheck struct {
	statefulSetChecker     StatefulSetStateChecker
	deploymentStateChecker DeploymentStateChecker
}

type ManagerKind string

const (
	DeploymentKind  ManagerKind = "Deployment"
	StatefulSetKind ManagerKind = "StatefulSet"
)

type Manager struct {
	Kind ManagerKind
	*apiappsv1.Deployment
	*apiappsv1.StatefulSet
}

func NewManagerStateCheck() *ManagerStateCheck {
	return &ManagerStateCheck{
		statefulSetChecker:     NewStatefulSetStateCheck(),
		deploymentStateChecker: NewDeploymentStateCheck(),
	}
}

func (m *ManagerStateCheck) GetState(ctx context.Context,
	clnt declarativev2.Client,
	resources []*resource.Info,
) (shared.State, error) {
	mgr := findManager(clnt, resources)
	if mgr == nil {
		return shared.StateReady, nil
	}

	switch mgr.Kind {
	case StatefulSetKind:
		return m.statefulSetChecker.GetState(ctx, clnt, mgr.StatefulSet)
	case DeploymentKind:
		return m.deploymentStateChecker.GetState(mgr.Deployment)
	}

	return shared.StateReady, nil
}

func findManager(clt declarativev2.Client, resources []*resource.Info) *Manager {
	deploy := &apiappsv1.Deployment{}
	statefulSet := &apiappsv1.StatefulSet{}

	for _, res := range resources {
		if err := clt.Scheme().Convert(res.Object, deploy, nil); err == nil {
			return &Manager{
				Kind:       DeploymentKind,
				Deployment: deploy,
			}
		}

		if err := clt.Scheme().Convert(res.Object, statefulSet, nil); err == nil {
			return &Manager{
				Kind:        StatefulSetKind,
				StatefulSet: statefulSet,
			}
		}
	}

	return nil
}
