package gatewaysecret

import (
	"context"
	"time"

	apicorev1 "k8s.io/api/core/v1"
)

const (
	TLSCrt     = "tls.crt"
	TLSKey     = "tls.key"
	CACrt      = "ca.crt"
	SecretKind = "Secret"
)

type Client interface {
	GetWatcherServingCertValidity(ctx context.Context) (time.Time, time.Time, error)
	GetGatewaySecret(ctx context.Context) (*apicorev1.Secret, error)
	CreateGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error
	UpdateGatewaySecret(ctx context.Context, secret *apicorev1.Secret) error
}

type TimeParserFunc func(secret *apicorev1.Secret, annotation string) (time.Time, error)

type Handler interface {
	ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error
}
