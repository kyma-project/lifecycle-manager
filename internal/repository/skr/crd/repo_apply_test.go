package crd_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	errorsinternal "github.com/kyma-project/lifecycle-manager/internal/errors"
	skrcrdrepo "github.com/kyma-project/lifecycle-manager/internal/repository/skr/crd"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestApply_PatchesCrdViaSSA(t *testing.T) {
	kymaName := types.NamespacedName{Name: random.Name(), Namespace: random.Name()}
	crdName := random.Name()

	patchClient := &applyClientStub{}
	clientRetrieverStub := &skrClientRetrieverStub{client: patchClient}
	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), crdName)

	kcpCrd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: apimetav1.ObjectMeta{Name: crdName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: shared.OperatorGroup,
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.WebhookConverter,
			},
		},
	}

	require.NoError(t, repo.Apply(t.Context(), kymaName, kcpCrd))

	assert.True(t, patchClient.called)
	assert.Equal(t, kymaName, clientRetrieverStub.receivedKey)

	patched, ok := patchClient.patched.(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, crdName, patched.GetName())
	assert.Equal(t, apiextensionsv1.SchemeGroupVersion.WithKind(
		reflect.TypeFor[apiextensionsv1.CustomResourceDefinition]().Name()), patched.GroupVersionKind())
	assert.Equal(t, shared.ManagedByLabelValue, patched.GetLabels()[shared.ManagedBy])

	conversion, found, err := unstructured.NestedString(patched.Object, "spec", "conversion", "strategy")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, string(apiextensionsv1.NoneConverter), conversion)

	require.Contains(t, patchClient.options, client.ForceOwnership)
	require.Contains(t, patchClient.options, fieldowners.LegacyLifecycleManager)
}

func TestApply_ClientNotFound_ReturnsError(t *testing.T) {
	clientRetrieverStub := &skrClientRetrieverStub{client: nil}
	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), random.Name())

	err := repo.Apply(context.Background(),
		types.NamespacedName{Name: random.Name(), Namespace: random.Name()},
		&apiextensionsv1.CustomResourceDefinition{})

	require.ErrorIs(t, err, errorsinternal.ErrSkrClientNotFound)
}

func TestApply_PatchError_ReturnsWrappedError(t *testing.T) {
	patchErr := errors.New("patch failed")
	patchClient := &applyClientStub{err: patchErr}
	clientRetrieverStub := &skrClientRetrieverStub{client: patchClient}
	repo := skrcrdrepo.NewRepository(clientRetrieverStub.retrieverFunc(), random.Name())

	err := repo.Apply(context.Background(),
		types.NamespacedName{Name: random.Name(), Namespace: random.Name()},
		&apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: apimetav1.ObjectMeta{Name: random.Name()},
		})

	require.ErrorIs(t, err, patchErr)
}

type applyClientStub struct {
	client.Client

	called  bool
	patched client.Object
	options []client.PatchOption
	err     error
}

func (c *applyClientStub) Patch(_ context.Context,
	obj client.Object, _ client.Patch, opts ...client.PatchOption,
) error {
	c.called = true
	c.patched = obj
	c.options = opts
	return c.err
}
