package statecheck

import (
	"context"

	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ManagerStateCheck struct {
	statefulSetChecker     StatefulSetStateChecker
	deploymentStateChecker DeploymentStateChecker
}

type DeploymentStateChecker interface {
	GetState(deploy *apiappsv1.Deployment) (shared.State, error)
}

type StatefulSetStateChecker interface {
	GetState(ctx context.Context, clnt client.Client, statefulSet *apiappsv1.StatefulSet) (shared.State, error)
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

func NewManagerStateCheck(statefulSetChecker StatefulSetStateChecker,
	deploymentChecker DeploymentStateChecker) *ManagerStateCheck {
	return &ManagerStateCheck{
		statefulSetChecker:     statefulSetChecker,
		deploymentStateChecker: deploymentChecker,
	}
}

func (m *ManagerStateCheck) GetState(ctx context.Context,
	clnt client.Client,
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

func findManager(clt client.Client, resources []*resource.Info) *Manager {
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
