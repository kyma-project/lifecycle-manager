package kyma_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
)

const (
	kymaName      = "kyma-123"
	kymaNamespace = "kcp-system"
)

var errGeneric = errors.New("generic error")

func Test_Get_WhenKymaNotFound_ReturnNotFoundError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&readerStubKymaNotFound{}, kymaNamespace)
	_, err := kymaClient.Get(t.Context(), kymaName)

	require.Error(t, err)
	require.True(t, apierrors.IsNotFound(err))
}

func Test_Get_WhenReaderReturnsError_ReturnError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&readerStubGenericError{}, kymaNamespace)
	_, err := kymaClient.Get(t.Context(), kymaName)

	require.Error(t, err)
	require.False(t, apierrors.IsNotFound(err))
	require.ErrorIs(t, err, errGeneric)
}

func Test_Get_WhenKymaFound_ReturnNoError(t *testing.T) {
	kymaClient := kymarepo.NewRepository(&readerStubValidKyma{}, kymaNamespace)
	foundKyma, err := kymaClient.Get(t.Context(), kymaName)

	require.NoError(t, err)
	require.Equal(t, kymaName, foundKyma.GetName())
	require.Equal(t, kymaNamespace, foundKyma.GetNamespace())
}

// Reader stubs

type readerStubKymaNotFound struct {
	client.Client
}

func (c *readerStubKymaNotFound) Get(_ context.Context, key client.ObjectKey, _ client.Object, _ ...client.GetOption,
) error {
	return apierrors.NewNotFound(schema.GroupResource{
		Group:    "v1beta2",
		Resource: "kyma",
	}, key.Name)
}

func (c *readerStubKymaNotFound) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return nil
}

type readerStubGenericError struct {
	client.Client
}

func (c *readerStubGenericError) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption,
) error {
	return errGeneric
}

func (c *readerStubGenericError) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return errGeneric
}

type readerStubValidKyma struct {
	client.Client

	listItems []client.Object
}

func (c *readerStubValidKyma) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption,
) error {
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)
	return nil
}

func (c *readerStubValidKyma) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	kymaList, ok := list.(*v1beta2.KymaList)
	if !ok {
		return fmt.Errorf("expected KymaList but got %T", list)
	}
	for _, item := range c.listItems {
		kymaItem, ok := item.(*v1beta2.Kyma)
		if !ok {
			return fmt.Errorf("expected KymaList but got %T", item)
		}
		kymaList.Items = append(kymaList.Items, *kymaItem)
	}
	return nil
}
