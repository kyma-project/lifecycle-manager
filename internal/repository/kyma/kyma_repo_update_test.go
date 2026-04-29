package kyma_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
)

func Test_Update_WhenClientReturnsNotFound_ReturnNotFoundError(t *testing.T) {
	notFoundErr := apierrors.NewNotFound(schema.GroupResource{
		Group:    "v1beta2",
		Resource: "kyma",
	}, kymaName)
	kymaClient := kymarepo.NewRepository(&writerStub{err: notFoundErr}, kymaNamespace)
	kyma := newKyma(kymaName, kymaNamespace)

	err := kymaClient.Update(t.Context(), kyma)

	require.Error(t, err)
	require.True(t, apierrors.IsNotFound(err))
}

func Test_Update_WhenClientReturnsGenericError_ReturnError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&writerStub{err: assert.AnError}, kymaNamespace)
	kyma := newKyma(kymaName, kymaNamespace)

	err := kymaClient.Update(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, assert.AnError)
}

func Test_Update_WhenClientSucceeds_ReturnNoError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&writerStub{}, kymaNamespace)
	kyma := newKyma(kymaName, kymaNamespace)

	err := kymaClient.Update(t.Context(), kyma)

	require.NoError(t, err)
}

// Helper functions

func newKyma(name, namespace string) *v1beta2.Kyma {
	return &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// Writer stub

type writerStub struct {
	client.Client

	err error
}

func (c *writerStub) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return c.err
}
