package kyma_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
)

func Test_List_WhenKymaNotFound_ReturnEmptyList(t *testing.T) {
	repo := kymarepo.NewRepository(&readerStubKymaNotFound{}, kymaNamespace)
	res, err := repo.LookupByLabel(t.Context(), "some-label", "some-value")

	require.NoError(t, err)
	assert.Empty(t, res)
}

func Test_List_WhenReaderReturnsError_ReturnError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&readerStubGenericError{}, kymaNamespace)
	_, err := kymaClient.LookupByLabel(t.Context(), "some-label", "some-value")

	require.Error(t, err)
	require.False(t, apierrors.IsNotFound(err))
	require.ErrorIs(t, err, errGeneric)
}

func Test_Get_WhenSingleKymaFound_ReturnsList(t *testing.T) {
	expectedKymas := make([]client.Object, 0, 1)
	expectedKymas = append(expectedKymas, testKyma(kymaName+"-1", kymaNamespace))
	kymaClient := kymarepo.NewRepository(&readerStubValidKyma{listItems: expectedKymas}, kymaNamespace)
	foundKymas, err := kymaClient.LookupByLabel(t.Context(), "some-label", "some-value")

	require.NoError(t, err)
	require.Len(t, foundKymas.Items, 1)
	require.Equal(t, kymaName+"-1", foundKymas.Items[0].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[0].GetNamespace())
}

func Test_Get_WhenTwoKymasFound_ReturnsList(t *testing.T) {
	// given
	expectedKymas := make([]client.Object, 0, 2)
	expectedKymas = append(expectedKymas, testKyma(kymaName+"-1", kymaNamespace))
	expectedKymas = append(expectedKymas, testKyma(kymaName+"-2", kymaNamespace))
	kymaClient := kymarepo.NewRepository(&readerStubValidKyma{listItems: expectedKymas}, kymaNamespace)

	// when
	foundKymas, err := kymaClient.LookupByLabel(t.Context(), "some-label", "some-value")

	// then
	require.NoError(t, err)
	require.Len(t, foundKymas.Items, 2)
	require.Equal(t, kymaName+"-1", foundKymas.Items[0].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[0].GetNamespace())
	require.Equal(t, kymaName+"-2", foundKymas.Items[1].GetName())
	require.Equal(t, kymaNamespace, foundKymas.Items[1].GetNamespace())
}

func testKyma(name, namespace string) *v1beta2.Kyma {
	return &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
