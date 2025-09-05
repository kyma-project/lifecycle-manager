package secret_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretrepository "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	secretName = random.Name()
	namespace  = random.Name()
)

func TestList_ClientCallSucceeds_ReturnsSecrets(t *testing.T) {
	secret1 := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "secret1",
			Namespace: namespace,
			Labels:    map[string]string{"app": "test"},
		},
	}
	secret2 := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "secret2",
			Namespace: namespace,
			Labels:    map[string]string{"app": "prod"},
		},
	}
	clientStub := &listClientStub{
		list: &apicorev1.SecretList{Items: []apicorev1.Secret{*secret1, *secret2}},
	}
	secretRepository := secretrepository.NewRepository(clientStub, namespace)

	result, err := secretRepository.List(t.Context(), k8slabels.SelectorFromSet(k8slabels.Set{"app": "test"}))

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "secret1", result.Items[0].Name)
	assert.True(t, clientStub.called)
}

type listClientStub struct {
	client.Client
	called bool
	list   *apicorev1.SecretList
}

func (c *listClientStub) List(_ context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	c.called = true
	c.list.DeepCopyInto(obj.(*apicorev1.SecretList))
	return nil
}
