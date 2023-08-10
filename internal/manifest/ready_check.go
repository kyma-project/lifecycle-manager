package manifest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	deploymentutil "k8s.io/kubectl/pkg/util/deployment"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

const customResourceStatePath = "status.state"

// NewCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewCustomResourceReadyCheck() *CustomResourceReadyCheck {
	return &CustomResourceReadyCheck{}
}

type CustomResourceReadyCheck struct{}

var (
	ErrNoDeterminedState     = errors.New("could not determine state")
	ErrCRInUnexpectedState   = errors.New("module CR in unexpected state during readiness check")
	ErrModuleDeploymentState = errors.New("module operator deployment is not ready")
)

func (c *CustomResourceReadyCheck) Run(ctx context.Context,
	clnt declarative.Client,
	obj declarative.Object,
	resources []*resource.Info,
) (declarative.StateInfo, error) {
	if !isDeploymentReady(clnt, resources) {
		return declarative.StateInfo{
				State: declarative.StateProcessing,
				Info:  ErrModuleDeploymentState.Error(),
			},
			ErrModuleDeploymentState
	}
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return declarative.StateInfo{State: declarative.StateError}, v1beta2.ErrTypeAssertManifest
	}
	if manifest.Spec.Resource == nil {
		return declarative.StateInfo{State: declarative.StateReady}, nil
	}
	res := manifest.Spec.Resource.DeepCopy()
	if err := clnt.Get(ctx, client.ObjectKeyFromObject(res), res); err != nil {
		return declarative.StateInfo{State: declarative.StateError},
			fmt.Errorf("failed to fetch resource: %w", err)
	}
	stateFromCR, stateExists, err := unstructured.NestedString(res.Object,
		strings.Split(customResourceStatePath, ".")...)
	if err != nil {
		return declarative.StateInfo{State: declarative.StateError},
			fmt.Errorf("could not get state from module CR %s at path %s to determine readiness: %w",
				res.GetName(), customResourceStatePath, ErrNoDeterminedState,
			)
	}

	// CR state might not been initialized, put manifest state into processing
	if !stateExists {
		return declarative.StateInfo{State: declarative.StateProcessing, Info: "module CR state not found"}, nil
	}

	typedState := declarative.State(stateFromCR)

	if !typedState.IsSupportedState() {
		return declarative.StateInfo{
			State: declarative.StateWarning,
			Info:  "module CR state is not supported, this module might not be default kyma module",
		}, nil
	}

	if typedState == declarative.StateDeleting || typedState == declarative.StateError {
		return declarative.StateInfo{State: typedState}, ErrCRInUnexpectedState
	}
	return declarative.StateInfo{State: typedState}, nil
}

func isDeploymentReady(clt declarative.Client, resources []*resource.Info) bool {
	deploy := &appsv1.Deployment{}
	found := false
	for _, res := range resources {
		err := clt.Scheme().Convert(res.Object, deploy, nil)
		if err == nil {
			found = true
			break
		}
	}
	// not every module operator use Deployment by default, e.g: StatefulSet also a valid approach
	if !found {
		return true
	}
	availableCond := deploymentutil.GetDeploymentCondition(deploy.Status, appsv1.DeploymentAvailable)
	if availableCond != nil && availableCond.Status == corev1.ConditionTrue {
		return true
	}
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return true
	}
	return false
}
