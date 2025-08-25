package statecheck

import (
	"context"
	"errors"

	apiappsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
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

var (
	ErrNoManagerProvided = errors.New("failed to find manager in provided resources")
	ErrNoStateDetermined = errors.New("failed to determine state for manager")
)

type Manager struct {
	kind        ManagerKind
	deployment  *apiappsv1.Deployment
	statefulSet *apiappsv1.StatefulSet
}

func NewManagerStateCheck(statefulSetChecker StatefulSetStateChecker,
	deploymentChecker DeploymentStateChecker,
) *ManagerStateCheck {
	return &ManagerStateCheck{
		statefulSetChecker:     statefulSetChecker,
		deploymentStateChecker: deploymentChecker,
	}
}

// Determines the state based on the manager. The manager may either be a Deployment or a StatefulSet and
// must be included in the provided resources.
// Will be refactored with https://github.com/kyma-project/lifecycle-manager/issues/1831
func (m *ManagerStateCheck) GetState(ctx context.Context,
	clnt client.Client,
	resources []*resource.Info,
) (shared.State, error) {
	mgr := findManager(clnt, resources)
	if mgr == nil {
		return shared.StateReady, nil
	}

	switch mgr.kind {
	case StatefulSetKind:
		return m.statefulSetChecker.GetState(ctx, clnt, mgr.statefulSet)
	case DeploymentKind:
		return m.deploymentStateChecker.GetState(mgr.deployment)
	}

	// fall through that should not be reached
	return shared.StateReady, nil
}

func findManager(clt client.Client, resources []*resource.Info) *Manager {
	deploy := &apiappsv1.Deployment{}
	statefulSet := &apiappsv1.StatefulSet{}

	for _, res := range resources {
		err := clt.Scheme().Convert(res.Object, deploy, nil)
		if err == nil {
			return &Manager{
				kind:       DeploymentKind,
				deployment: deploy,
			}
		}

		err = clt.Scheme().Convert(res.Object, statefulSet, nil)
		if err == nil {
			return &Manager{
				kind:        StatefulSetKind,
				statefulSet: statefulSet,
			}
		}
	}

	return nil
}
