package v1beta1

import (
	"context"
	"encoding/json"
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

const customResourceStatePath = "status.state"

// NewManifestCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewManifestCustomResourceReadyCheck() *ManifestCustomResourceReadyCheck {
	return &ManifestCustomResourceReadyCheck{}
}

type ManifestCustomResourceReadyCheck struct{}

var (
	ErrNoDeterminedState   = errors.New("could not determine state")
	ErrCRInUnexpectedState = errors.New("module CR in unexpected state during readiness check")
)

func (c *ManifestCustomResourceReadyCheck) Run(ctx context.Context,
	clnt declarative.Client,
	obj declarative.Object,
	resources []*resource.Info,
) (declarative.StateInfo, error) {
	if !isDeploymentReady(clnt, resources) {
		return declarative.StateInfo{State: declarative.StateProcessing, Info: "module operator deployment is not ready"}, nil
	}
	manifest := obj.(*v1beta2.Manifest)
	if manifest.Spec.Resource == nil {
		return declarative.StateInfo{State: declarative.StateReady}, nil
	}
	res := manifest.Spec.Resource.DeepCopy()
	if err := clnt.Get(ctx, client.ObjectKeyFromObject(res), res); err != nil {
		return declarative.StateInfo{State: declarative.StateError}, err
	}
	customStateCheck, err := parseCustomStateCheck(manifest)
	if err != nil {
		return declarative.StateInfo{State: declarative.StateError}, err
	}
	stateFromCR, stateExists, err := unstructured.NestedString(res.Object,
		strings.Split(customStateCheck.JSONPath, ".")...)
	// CR state might not been initialized, put manifest state into processing
	if !stateExists {
		return declarative.StateInfo{State: declarative.StateProcessing, Info: "module CR state not found"}, nil
	}

	if err != nil {
		return declarative.StateInfo{State: declarative.StateError}, fmt.Errorf(
			"could not get state from module CR %s at path %s to determine readiness: %w",
			res.GetName(), customStateCheck.JSONPath, ErrNoDeterminedState,
		)
	}

	typedState := declarative.State(stateFromCR)
	if customStateCheck.Value == stateFromCR {
		typedState = declarative.StateReady
	}

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

func parseCustomStateCheck(manifest *v1beta2.Manifest) (v1beta2.CustomStateCheck, error) {
	customStateCheckAnnotation, found := manifest.Annotations[v1beta2.CustomStateCheckAnnotation]
	if !found {
		return v1beta2.CustomStateCheck{JSONPath: customResourceStatePath, Value: string(v1beta2.StateReady)}, nil
	}
	customStateCheck := v1beta2.CustomStateCheck{}
	if err := json.Unmarshal([]byte(customStateCheckAnnotation), &customStateCheck); err != nil {
		return customStateCheck, err
	}
	return customStateCheck, nil
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
