package gatewaysecret_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
)

func TestNewGatewaySecret(t *testing.T) {
	t.Parallel()

	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte(newTLSCertValue),
			"tls.key": []byte(newTLSKeyValue),
			"ca.crt":  []byte(newCACertValue),
		},
	}
	gwSecret := gatewaysecret.NewGatewaySecret(rootSecret)

	require.Equal(t, "klm-istio-gateway", gwSecret.Name)
	require.Equal(t, "istio-system", gwSecret.Namespace)

	require.Equal(t, newTLSCertValue, string(gwSecret.Data["tls.crt"]))
	require.Equal(t, newTLSKeyValue, string(gwSecret.Data["tls.key"]))
	require.Equal(t, newCACertValue, string(gwSecret.Data["ca.crt"]))
}
