package kyma_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
)

func Test_GetAll_WhenNoKymasExist_ReturnsEmptyList(t *testing.T) {
	repo := kymarepo.NewRepository(&readerStubKymaNotFound{}, kymaNamespace)
	res, err := repo.GetAll(t.Context())

	require.NoError(t, err)
	assert.Empty(t, res.Items)
}

func Test_GetAll_WhenClientReturnsError_ReturnsError(t *testing.T) {
	repo := kymarepo.NewRepository(&readerStubGenericError{}, kymaNamespace)
	_, err := repo.GetAll(t.Context())

	require.Error(t, err)
	require.ErrorIs(t, err, errGeneric)
}

func Test_GetAll_WhenSingleKymaExists_ReturnsList(t *testing.T) {
	expectedKymas := []client.Object{testKyma(kymaName + "-1")}
	repo := kymarepo.NewRepository(&readerStubValidKyma{listItems: expectedKymas}, kymaNamespace)

	foundKymas, err := repo.GetAll(t.Context())

	require.NoError(t, err)
	require.Len(t, foundKymas.Items, 1)
	require.Equal(t, kymaName+"-1", foundKymas.Items[0].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[0].GetNamespace())
}

func Test_GetAll_WhenMultipleKymasExist_ReturnsList(t *testing.T) {
	expectedKymas := []client.Object{
		testKyma(kymaName + "-1"),
		testKyma(kymaName + "-2"),
	}
	repo := kymarepo.NewRepository(&readerStubValidKyma{listItems: expectedKymas}, kymaNamespace)

	foundKymas, err := repo.GetAll(t.Context())

	require.NoError(t, err)
	require.Len(t, foundKymas.Items, 2)
	require.Equal(t, kymaName+"-1", foundKymas.Items[0].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[0].GetNamespace())
	require.Equal(t, kymaName+"-2", foundKymas.Items[1].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[1].GetNamespace())
}
