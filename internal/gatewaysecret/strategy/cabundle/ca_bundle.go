package cabundle

import (
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/strategy"
	apicorev1 "k8s.io/api/core/v1"
)

type Strategy struct{}

func (Strategy) RotateGatewaySecret(rootSecret *apicorev1.Secret, gatewaySecret *apicorev1.Secret) {
	if gatewaySecret.Data == nil {
		gatewaySecret.Data = make(map[string][]byte)
	}
	gatewaySecret.Data[strategy.TLSCrt] = rootSecret.Data[strategy.TLSCrt]
	gatewaySecret.Data[strategy.TLSKey] = rootSecret.Data[strategy.TLSKey]
	gatewaySecret.Data[strategy.CACrt] = append(rootSecret.Data[strategy.CACrt], gatewaySecret.Data[strategy.CACrt]...)
}
