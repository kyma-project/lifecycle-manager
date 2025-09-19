package keychainprovider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
)

func TestGetWhenClientReturnsNotFoundErrorReturnsErrNoAuthSecretFoundAndSecretName(t *testing.T) {
	kcpClient := fake.NewFakeClient()
	secretName := types.NamespacedName{Name: "test-secret", Namespace: "test-namespace"}
	sut := keychainprovider.NewFromSecretKeyChainProvider(kcpClient, secretName)

	keychain, err := sut.Get(t.Context())

	assert.Nil(t, keychain)
	require.ErrorIs(t, err, keychainprovider.ErrNoAuthSecretFound)
	require.ErrorContains(t, err, secretName.String())
}

func TestGetWhenClientReturnsOtherErrorReturnsFailedToGetError(t *testing.T) {
	kcpClient := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: otherErrorFunc,
	}).Build()
	secretName := types.NamespacedName{Name: "test-secret", Namespace: "test-namespace"}
	sut := keychainprovider.NewFromSecretKeyChainProvider(kcpClient, secretName)

	keychain, err := sut.Get(t.Context())

	assert.Nil(t, keychain)
	require.Error(t, err)
	require.NotErrorIs(t, err, keychainprovider.ErrNoAuthSecretFound)
	require.ErrorContains(t, err, "failed to get oci cred secret")
}

func TestGetWhenClientReturnsSecretReturnsKeychain(t *testing.T) {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
	}
	kcpClient := fake.NewClientBuilder().WithObjects(secret).Build()

	sut := keychainprovider.NewFromSecretKeyChainProvider(
		kcpClient,
		types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace},
	)

	keychain, err := sut.Get(t.Context())

	assert.NotEmpty(t, keychain)
	require.NoError(t, err)
}

var otherErrorFunc = func(ctx context.Context, client client.WithWatch,
	key client.ObjectKey, obj client.Object, opts ...client.GetOption,
) error {
	return errors.New("some other error")
}
