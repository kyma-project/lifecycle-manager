package legacy

import (
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/strategy"
)

type Strategy struct{}

func (Strategy) RotateGatewaySecret(rootSecret *apicorev1.Secret, gatewaySecret *apicorev1.Secret) {
	if gatewaySecret.Data == nil {
		gatewaySecret.Data = make(map[string][]byte)
	}
	gatewaySecret.Data[strategy.TLSCrt] = rootSecret.Data[strategy.TLSCrt]
	gatewaySecret.Data[strategy.TLSKey] = rootSecret.Data[strategy.TLSKey]
	gatewaySecret.Data[strategy.CACrt] = rootSecret.Data[strategy.CACrt]
}
