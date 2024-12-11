package gatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

// TODO Move this to consumer

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

var errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")

var ParseLastModifiedFunc TimeParserFunc = func(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[shared.LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errCouldNotGetLastModifiedAt
}
