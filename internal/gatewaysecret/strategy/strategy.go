package strategy

import (
	apicorev1 "k8s.io/api/core/v1"
)

const (
	TLSCrt = "tls.crt"
	TLSKey = "tls.key"
	CACrt  = "ca.crt"
)

type SecretRotationStrategy interface {
	RotateGatewaySecret(rootSecret *apicorev1.Secret, gatewaySecret *apicorev1.Secret)
}
