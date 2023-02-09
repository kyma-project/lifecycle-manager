package v1beta1

import (
	"context"
	"errors"
	"fmt"
	"strings"

	manifestv1beta1 "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	declarative "github.com/kyma-project/lifecycle-manager/pkg/declarative/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const customResourceStatePath = "status.state"

var ErrCustomResourceStateNotFound = errors.New("custom resource state not found")

// NewManifestCustomResourceReadyCheck creates a readiness check that verifies that the Resource in the Manifest
// returns the ready state, if not it returns not ready.
func NewManifestCustomResourceReadyCheck() *ManifestCustomResourceReadyCheck {
	return &ManifestCustomResourceReadyCheck{}
}

type ManifestCustomResourceReadyCheck struct{}

var ErrNoDeterminedState = errors.New("could not determine state")

func (c *ManifestCustomResourceReadyCheck) Run(
	ctx context.Context, clnt declarative.Client, obj declarative.Object, _ []*resource.Info,
) error {
	manifest := obj.(*manifestv1beta1.Manifest)
	if manifest.Spec.Resource == nil {
		return nil
	}
	res := manifest.Spec.Resource.DeepCopy()
	if err := clnt.Get(ctx, client.ObjectKeyFromObject(res), res); err != nil {
		return err
	}
	state, stateExists, err := unstructured.NestedString(res.Object, strings.Split(customResourceStatePath, ".")...)
	if err != nil {
		return fmt.Errorf(
			"could not get state from custom resource %s at path %s to determine readiness: %w",
			res.GetName(), customResourceStatePath, ErrNoDeterminedState,
		)
	}
	if !stateExists {
		return ErrCustomResourceStateNotFound
	}

	if state := declarative.State(state); state != declarative.StateReady {
		return fmt.Errorf(
			"custom resource state is %s but expected %s: %w", state, declarative.StateReady,
			declarative.ErrResourcesNotReady,
		)
	}

	return nil
}
