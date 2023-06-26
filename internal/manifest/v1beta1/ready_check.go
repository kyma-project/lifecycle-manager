package v1beta1

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"sigs.k8s.io/controller-runtime/pkg/client"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

const customResourceStatePath = "status.status"

// NewManifestCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewManifestCustomResourceReadyCheck() *ManifestCustomResourceReadyCheck {
	return &ManifestCustomResourceReadyCheck{}
}

type ManifestCustomResourceReadyCheck struct{}

var ErrNoDeterminedState = errors.New("could not determine state")

func (c *ManifestCustomResourceReadyCheck) Run(
	ctx context.Context, clnt declarative.Client, obj declarative.Object, resources []*resource.Info,
) (declarative.State, error) {
	if err := checkDeploymentState(clnt, resources); err != nil {
		return declarative.StateError, err
	}
	manifest := obj.(*v1beta2.Manifest)
	if manifest.Spec.Resource == nil {
		return declarative.StateReady, nil
	}
	res := manifest.Spec.Resource.DeepCopy()
	if err := clnt.Get(ctx, client.ObjectKeyFromObject(res), res); err != nil {
		return declarative.StateError, err
	}
	state, stateExists, err := unstructured.NestedString(res.Object, strings.Split(customResourceStatePath, ".")...)
	if err != nil {
		return declarative.StateError, fmt.Errorf(
			"could not get state from custom resource %s at path %s to determine readiness: %w",
			res.GetName(), customResourceStatePath, ErrNoDeterminedState,
		)
	}
	if !stateExists {
		return declarative.StateError, declarative.ErrCustomResourceStateNotFound
	}
	typedState := declarative.State(state)
	if !stableState(typedState) {
		return declarative.StateError, fmt.Errorf(
			"custom resource state is %s: %w", state, declarative.ErrResourcesNotReady,
		)
	}

	return typedState, nil
}

func stableState(state declarative.State) bool {
	return state == declarative.StateReady || state == declarative.StateWarning
}

var ErrDeploymentResNotFound = errors.New("deployment resource is not found")

func checkDeploymentState(clt declarative.Client, resources []*resource.Info) error {
	deploy := &appsv1.Deployment{}
	found := false
	for _, res := range resources {
		err := clt.Scheme().Convert(res.Object, deploy, nil)
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return ErrDeploymentResNotFound
	}
	availableCond := deploymentutil.GetDeploymentCondition(deploy.Status, appsv1.DeploymentAvailable)
	if availableCond != nil && availableCond.Status == corev1.ConditionTrue {
		return nil
	}
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return nil
	}
	return fmt.Errorf("%w: (ns=%s, name=%s)", declarative.ErrDeploymentNotReady, deploy.Namespace, deploy.Name)
}
