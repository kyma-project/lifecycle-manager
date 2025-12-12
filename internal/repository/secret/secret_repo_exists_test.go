package secret_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestExists_ClientCallSucceeds_ReturnsExists(t *testing.T) {
	kymaName := random.Name()
	repoNamespace := random.Name()

	clientStub := &getClientStub{
		object: &apicorev1.Secret{
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
		},
	}
	secretRepository := secretrepo.NewRepository(clientStub, repoNamespace)

	result, err := secretRepository.Exists(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t,
		client.ObjectKey{Name: kymaName, Namespace: repoNamespace},
		clientStub.key)
}

func TestExists_ClientCallFailsWithNotFound_ReturnsNotExists(t *testing.T) {
	kymaName := random.Name()
	repoNamespace := random.Name()

	clientStub := &getClientStub{
		err: apierrors.NewNotFound(apicorev1.Resource("secrets"), kymaName),
	}
	secretRepository := secretrepo.NewRepository(clientStub, repoNamespace)

	result, err := secretRepository.Exists(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t,
		client.ObjectKey{Name: kymaName, Namespace: repoNamespace},
		clientStub.key)
}

func TestExists_ClientReturnsAnError_ReturnsError(t *testing.T) {
	clientStub := &getClientStub{
		err: assert.AnError,
	}
	secretRepository := secretrepo.NewRepository(clientStub, random.Name())

	result, err := secretRepository.Exists(t.Context(), random.Name())

	assert.True(t, result)
	require.ErrorIs(t, err, assert.AnError)
	assert.True(t, clientStub.called)
}
