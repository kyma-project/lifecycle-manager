package gatewaysecret

import (
	"context"
	"fmt"
	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	
	"github.com/kyma-project/lifecycle-manager/api/shared"
)

func GetGatewaySecret(ctx context.Context, clnt client.Client) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      shared.GatewaySecretName,
		Namespace: shared.IstioNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get gateway secret %s: %w", shared.GatewaySecretName, err)
	}
	return secret, nil
}
