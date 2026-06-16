package statecheck_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
)

func TestExistsStateCheck_ReturnsReadyWhenAllResourcesExist(t *testing.T) {
	t.Parallel()

	scheme := machineryruntime.NewScheme()
	require.NoError(t, apicorev1.AddToScheme(scheme))

	cm := &apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{Name: "cm", Namespace: "default"},
	}
	clnt := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	state, err := statecheck.NewExistsStateCheck().GetState(t.Context(), clnt,
		[]client.Object{&apicorev1.ConfigMap{ObjectMeta: apimetav1.ObjectMeta{Name: "cm", Namespace: "default"}}})

	require.NoError(t, err)
	assert.Equal(t, shared.StateReady, state)
}

func TestExistsStateCheck_TolerantToNotFoundResources(t *testing.T) {
	t.Parallel()

	scheme := machineryruntime.NewScheme()
	require.NoError(t, apicorev1.AddToScheme(scheme))
	clnt := fake.NewClientBuilder().WithScheme(scheme).Build()

	state, err := statecheck.NewExistsStateCheck().GetState(t.Context(), clnt,
		[]client.Object{&apicorev1.ConfigMap{ObjectMeta: apimetav1.ObjectMeta{Name: "missing", Namespace: "default"}}})

	require.NoError(t, err, "missing resources are tolerated via IgnoreNotFound")
	assert.Equal(t, shared.StateReady, state)
}

func TestExistsStateCheck_PropagatesNonNotFoundError(t *testing.T) {
	t.Parallel()

	scheme := machineryruntime.NewScheme()
	require.NoError(t, apicorev1.AddToScheme(scheme))

	apiErr := apierrors.NewInternalError(errors.New("boom"))
	clnt := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object,
			_ ...client.GetOption,
		) error {
			return apiErr
		},
	}).Build()

	state, err := statecheck.NewExistsStateCheck().GetState(t.Context(), clnt,
		[]client.Object{&apicorev1.ConfigMap{ObjectMeta: apimetav1.ObjectMeta{Name: "cm", Namespace: "default"}}})

	require.Error(t, err)
	assert.Equal(t, shared.StateError, state)
	assert.ErrorContains(t, err, "failed to fetch object by key")
}
