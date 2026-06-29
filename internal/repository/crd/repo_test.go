package crd_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/crd"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestGet_ReturnsCrd(t *testing.T) {
	expected := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: apimetav1.ObjectMeta{Name: random.Name()},
	}
	reader := &readerStub{crd: expected}
	repo := crd.NewRepository(reader)

	got, err := repo.Get(t.Context(), expected.Name)

	require.NoError(t, err)
	assert.Equal(t, expected.Name, got.Name)
	assert.Equal(t, expected.Name, reader.requestedKey.Name)
}

func TestGet_ReturnsError(t *testing.T) {
	expected := errors.New("read failed")
	reader := &readerStub{err: expected}
	repo := crd.NewRepository(reader)

	_, err := repo.Get(t.Context(), random.Name())

	require.ErrorIs(t, err, expected)
}

type readerStub struct {
	crd          *apiextensionsv1.CustomResourceDefinition
	err          error
	requestedKey types.NamespacedName
}

func (r *readerStub) Get(_ context.Context, key types.NamespacedName, obj client.Object, _ ...client.GetOption) error {
	r.requestedKey = key
	if r.err != nil {
		return r.err
	}
	if r.crd != nil {
		r.crd.DeepCopyInto(obj.(*apiextensionsv1.CustomResourceDefinition))
	}
	return nil
}
