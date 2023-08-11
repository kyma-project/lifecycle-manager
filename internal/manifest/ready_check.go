package manifest

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

// NewCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewCustomResourceReadyCheck() *CustomResourceReadyCheck {
	return &CustomResourceReadyCheck{}
}

type CustomResourceReadyCheck struct{}

var (
	ErrCRInUnexpectedState  = errors.New("module CR in unexpected state during readiness check")
	ErrNotSupportedState    = errors.New("module CR state not support")
	ErrRequiredStateMissing = errors.New("required Ready and Error state mapping are missing")
)

func (c *CustomResourceReadyCheck) Run(ctx context.Context,
	clnt declarative.Client,
	obj declarative.Object,
	resources []*resource.Info,
) (declarative.StateInfo, error) {
	if !isDeploymentReady(clnt, resources) {
		return declarative.StateInfo{
			State: declarative.StateProcessing,
			Info:  "module operator deployment is not ready",
		}, nil
	}
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return declarative.StateInfo{State: declarative.StateError}, v1beta2.ErrTypeAssertManifest
	}
	if manifest.Spec.Resource == nil {
		return declarative.StateInfo{State: declarative.StateReady}, nil
	}
	moduleCR := manifest.Spec.Resource.DeepCopy()
	if err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleCR), moduleCR); err != nil {
		return declarative.StateInfo{State: declarative.StateError}, fmt.Errorf("failed to fetch resource: %w", err)
	}
	return HandleState(manifest, moduleCR)
}

func HandleState(manifest *v1beta2.Manifest, moduleCR *unstructured.Unstructured) (declarative.StateInfo, error) {
	typedState, stateExists, err := mappingState(manifest, moduleCR)
	if err != nil {
		// Only happens for kyma module CR
		if errors.Is(err, ErrNotSupportedState) {
			return declarative.StateInfo{
				State: declarative.StateWarning,
				Info:  ErrNotSupportedState.Error(),
			}, nil
		}
		return declarative.StateInfo{State: declarative.StateError}, fmt.Errorf(
			"could not get state from module CR %s to determine readiness: %w",
			moduleCR.GetName(), err,
		)
	}

	// CR state might not been initialized, put manifest state into processing
	if !stateExists {
		return declarative.StateInfo{State: declarative.StateProcessing, Info: "module CR state not found"}, nil
	}

	if typedState == declarative.StateDeleting || typedState == declarative.StateError {
		return declarative.StateInfo{State: typedState}, ErrCRInUnexpectedState
	}
	return declarative.StateInfo{State: typedState}, nil
}

func mappingState(manifest *v1beta2.Manifest, moduleCR *unstructured.Unstructured) (declarative.State, bool, error) {
	stateChecks, customStateFound, err := parseStateChecks(manifest)
	if err != nil {
		return "", false, err
	}
	// make sure ready and error state exists, for other missing customized state, can be ignored.
	if requiredStateMissing(stateChecks) {
		return "", false, ErrRequiredStateMissing
	}
	stateResult := map[v1beta2.State]bool{}
	for _, stateCheck := range stateChecks {
		stateFromCR, stateExists, err := unstructured.NestedString(moduleCR.Object,
			strings.Split(stateCheck.JSONPath, ".")...)
		if err != nil {
			return "", false, fmt.Errorf("could not get state from module CR %s at path %s "+
				"to determine readiness: %w", moduleCR.GetName(), stateCheck.JSONPath, err)
		}
		if !stateExists {
			continue
		}
		if !customStateFound && !declarative.State(stateFromCR).IsSupportedState() {
			return "", false, ErrNotSupportedState
		}
		_, found := stateResult[stateCheck.MappedState]
		if found {
			stateResult[stateCheck.MappedState] = stateResult[stateCheck.MappedState] && stateFromCR == stateCheck.Value
		} else {
			stateResult[stateCheck.MappedState] = stateFromCR == stateCheck.Value
		}
	}
	return calculateFinalState(stateResult), true, nil
}

func calculateFinalState(stateResult map[v1beta2.State]bool) declarative.State {
	if stateResult[v1beta2.StateError] {
		return declarative.StateError
	}
	if stateResult[v1beta2.StateReady] {
		return declarative.StateReady
	}
	if stateResult[v1beta2.StateWarning] {
		return declarative.StateWarning
	}
	if stateResult[v1beta2.StateDeleting] {
		return declarative.StateDeleting
	}

	// by default, if ready/error state condition not match, assume module CR under processing
	return declarative.StateProcessing
}

func requiredStateMissing(stateChecks []*v1beta2.CustomStateCheck) bool {
	readyMissing := true
	errorMissing := true
	for _, stateCheck := range stateChecks {
		if stateCheck.MappedState == v1beta2.StateReady {
			readyMissing = false
		}
		if stateCheck.MappedState == v1beta2.StateError {
			errorMissing = false
		}
	}
	return readyMissing || errorMissing
}

func parseStateChecks(manifest *v1beta2.Manifest) ([]*v1beta2.CustomStateCheck, bool, error) {
	customStateCheckAnnotation, found := manifest.Annotations[v1beta2.CustomStateCheckAnnotation]
	if !found {
		return []*v1beta2.CustomStateCheck{
			{
				JSONPath:    customResourceStatePath,
				Value:       string(v1beta2.StateReady),
				MappedState: v1beta2.StateReady,
			},
			{
				JSONPath:    customResourceStatePath,
				Value:       string(v1beta2.StateError),
				MappedState: v1beta2.StateError,
			},
		}, false, nil
	}
	var stateCheck []*v1beta2.CustomStateCheck
	if err := json.Unmarshal([]byte(customStateCheckAnnotation), &stateCheck); err != nil {
		return stateCheck, true, fmt.Errorf("failed to unmarshal stateCheck: %w", err)
	}
	return stateCheck, true, nil
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
