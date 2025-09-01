package testutils

import (
	"context"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	secretrepository "github.com/kyma-project/lifecycle-manager/internal/repository/secret"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

func DeleteAccessSecret(ctx context.Context, clnt client.Client, kymaName string) error {
	secret, err := GetAccessSecret(ctx, clnt, kymaName)
	if util.IsNotFound(err) {
		return nil
	}
	return clnt.Delete(ctx, secret)
}

func AccessSecretExists(ctx context.Context, clnt client.Client, kymaName string) error {
	secret, err := GetAccessSecret(ctx, clnt, kymaName)
	return CRExists(secret, err)
}

func GetAccessSecret(ctx context.Context, clnt client.Client, name string) (*apicorev1.Secret, error) {
	accessSecretRepository := secretrepository.NewRepository(clnt, shared.DefaultControlPlaneNamespace)
	accessSecretService := accessmanager.NewService(accessSecretRepository)
	return accessSecretService.GetAccessSecretByKyma(ctx, name)
}

func CreateAccessSecret(ctx context.Context, clnt client.Client, name, patchedRuntimeConfig string) error {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: shared.DefaultControlPlaneNamespace,
			Labels: map[string]string{
				shared.KymaName: name,
			},
		},
		Data: map[string][]byte{"config": []byte(patchedRuntimeConfig)},
	}
	return clnt.Create(ctx, secret)
}
