package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/util/deployment"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	ToWarningDuration                = 5 * time.Minute
	customResourceStatePath          = "status.state"
	ModuleCRWithCustomCheckWarning   = "module CR state not found or given customStateCheck.jsonPath is not exists"
	ModuleCRWithNoCustomCheckWarning = "module CR state not found"
)

// NewCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewCustomResourceReadyCheck() *CustomResourceReadyCheck {
	return &CustomResourceReadyCheck{}
}

type CustomResourceReadyCheck struct{}

var (
	ErrNotSupportedState    = errors.New("module CR state not support")
	ErrRequiredStateMissing = errors.New("required Ready and Error state mapping are missing")
)

func (c *CustomResourceReadyCheck) Run(ctx context.Context,
	clnt declarativev2.Client,
	obj declarativev2.Object,
	resources []*resource.Info,
) (declarativev2.StateInfo, error) {
	if !isDeploymentReady(clnt, resources) {
		return declarativev2.StateInfo{
			State: shared.StateProcessing,
			Info:  "module operator deployment is not ready",
		}, nil
	}
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return declarativev2.StateInfo{State: shared.StateError}, pkgapiv1beta2.ErrTypeAssertManifest
	}
	if manifest.Spec.Resource == nil {
		return declarativev2.StateInfo{State: shared.StateReady}, nil
	}
	moduleCR := manifest.Spec.Resource.DeepCopy()

	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleCR), moduleCR)
	if err != nil {
		if util.IsNotFound(err) && !manifest.DeletionTimestamp.IsZero() {
			return declarativev2.StateInfo{State: shared.StateDeleting}, nil
		}
		return declarativev2.StateInfo{State: shared.StateError}, fmt.Errorf("failed to fetch resource: %w", err)
	}
	return HandleState(manifest, moduleCR)
}

func HandleState(manifest *v1beta2.Manifest, moduleCR *unstructured.Unstructured) (declarativev2.StateInfo, error) {
	stateChecks, customStateFound, err := parseStateChecks(manifest)
	if err != nil {
		return declarativev2.StateInfo{State: shared.StateError}, fmt.Errorf(
			"could not get state from module CR %s to determine readiness: %w",
			moduleCR.GetName(), err,
		)
	}
	typedState, stateExists, err := mappingState(stateChecks, moduleCR, customStateFound)
	if err != nil {
		// Only happens for kyma module CR
		if errors.Is(err, ErrNotSupportedState) {
			return declarativev2.StateInfo{
				State: shared.StateWarning,
				Info:  ErrNotSupportedState.Error(),
			}, nil
		}
		return declarativev2.StateInfo{State: shared.StateError}, fmt.Errorf(
			"could not get state from module CR %s to determine readiness: %w",
			moduleCR.GetName(), err,
		)
	}
	if !stateExists {
		info := ModuleCRWithNoCustomCheckWarning
		if customStateFound {
			info = ModuleCRWithCustomCheckWarning
		}
		state := shared.StateProcessing
		// If wait for certain period of time, state still not found, put manifest state into Warning
		if manifest.CreationTimestamp.Add(ToWarningDuration).Before(time.Now()) {
			state = shared.StateWarning
		}

		return declarativev2.StateInfo{State: state, Info: info}, nil
	}

	return declarativev2.StateInfo{State: typedState}, nil
}

func mappingState(stateChecks []*v1beta2.CustomStateCheck,
	moduleCR *unstructured.Unstructured,
	customStateFound bool,
) (shared.State, bool, error) {
	// make sure ready and error state exists, for other missing customized state, can be ignored.
	if requiredStateMissing(stateChecks) {
		return "", false, ErrRequiredStateMissing
	}
	stateResult := map[shared.State]bool{}
	foundStateInCR := false
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
		if !customStateFound && !shared.State(stateFromCR).IsSupportedState() {
			return "", false, ErrNotSupportedState
		}
		foundStateInCR = true
		_, found := stateResult[stateCheck.MappedState]
		if found {
			stateResult[stateCheck.MappedState] = stateResult[stateCheck.MappedState] && stateFromCR == stateCheck.Value
		} else {
			stateResult[stateCheck.MappedState] = stateFromCR == stateCheck.Value
		}
	}
	return calculateFinalState(stateResult), foundStateInCR, nil
}

func calculateFinalState(stateResult map[shared.State]bool) shared.State {
	if stateResult[shared.StateError] {
		return shared.StateError
	}
	if stateResult[shared.StateReady] {
		return shared.StateReady
	}
	if stateResult[shared.StateWarning] {
		return shared.StateWarning
	}
	if stateResult[shared.StateDeleting] {
		return shared.StateDeleting
	}

	// by default, if ready/error state condition not match, assume module CR under processing
	return shared.StateProcessing
}

func requiredStateMissing(stateChecks []*v1beta2.CustomStateCheck) bool {
	readyMissing := true
	errorMissing := true
	for _, stateCheck := range stateChecks {
		if stateCheck.MappedState == shared.StateReady {
			readyMissing = false
		}
		if stateCheck.MappedState == shared.StateError {
			errorMissing = false
		}
	}
	return readyMissing || errorMissing
}

func parseStateChecks(manifest *v1beta2.Manifest) ([]*v1beta2.CustomStateCheck, bool, error) {
	customStateCheckAnnotation, found := manifest.Annotations[shared.CustomStateCheckAnnotation]
	if !found {
		return []*v1beta2.CustomStateCheck{
			{
				JSONPath:    customResourceStatePath,
				Value:       string(shared.StateReady),
				MappedState: shared.StateReady,
			},
			{
				JSONPath:    customResourceStatePath,
				Value:       string(shared.StateError),
				MappedState: shared.StateError,
			},
			{
				JSONPath:    customResourceStatePath,
				Value:       string(shared.StateDeleting),
				MappedState: shared.StateDeleting,
			},
			{
				JSONPath:    customResourceStatePath,
				Value:       string(shared.StateWarning),
				MappedState: shared.StateWarning,
			},
		}, false, nil
	}
	var stateCheck []*v1beta2.CustomStateCheck
	if err := json.Unmarshal([]byte(customStateCheckAnnotation), &stateCheck); err != nil {
		return stateCheck, true, fmt.Errorf("failed to unmarshal stateCheck: %w", err)
	}
	return stateCheck, true, nil
}

func isDeploymentReady(clt declarativev2.Client, resources []*resource.Info) bool {
	deploy := &apiappsv1.Deployment{}
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
	availableCond := deployment.GetDeploymentCondition(deploy.Status, apiappsv1.DeploymentAvailable)
	if availableCond != nil && availableCond.Status == apicorev1.ConditionTrue {
		return true
	}
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == deploy.Status.ReadyReplicas {
		return true
	}
	return false
}
