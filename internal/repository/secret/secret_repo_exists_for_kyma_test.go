package secret_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	secretrepo "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestExistsForKyma_ClientCallSucceeds_ReturnsExists(t *testing.T) {
	kymaName := random.Name()
	repoNamespace := random.Name()
	selector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kymaName})

	clientStub := &listClientStub{
		list: &apicorev1.SecretList{
			Items: []apicorev1.Secret{
				{
					ObjectMeta: apimetav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
		},
	}
	secretRepository := secretrepo.NewRepository(clientStub, repoNamespace)

	result, err := secretRepository.ExistsForKyma(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, repoNamespace, clientStub.lastNamespace)
	assert.Equal(t, selector, clientStub.lastSelector)
}

func TestExistsForKyma_ClientCallSucceeds_ReturnsNotExists(t *testing.T) {
	kymaName := random.Name()
	repoNamespace := random.Name()
	selector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kymaName})

	clientStub := &listClientStub{
		list: &apicorev1.SecretList{
			Items: []apicorev1.Secret{},
		},
	}
	secretRepository := secretrepo.NewRepository(clientStub, repoNamespace)

	result, err := secretRepository.ExistsForKyma(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, repoNamespace, clientStub.lastNamespace)
	assert.Equal(t, selector, clientStub.lastSelector)
}

func TestExistsForKyma_ClientReturnsAnError_ReturnsError(t *testing.T) {
	kymaName := random.Name()
	repoNamespace := random.Name()
	selector := k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kymaName})

	clientStub := &listClientStub{
		err: assert.AnError,
	}
	secretRepository := secretrepo.NewRepository(clientStub, repoNamespace)

	result, err := secretRepository.ExistsForKyma(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	assert.False(t, result)
	assert.True(t, clientStub.called)
	assert.Equal(t, repoNamespace, clientStub.lastNamespace)
	assert.Equal(t, selector, clientStub.lastSelector)
}
