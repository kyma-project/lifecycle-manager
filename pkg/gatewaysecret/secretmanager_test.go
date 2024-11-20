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

	require.True(t, gwSecret.Name == "klm-istio-gateway")
	require.True(t, gwSecret.Namespace == "istio-system")

	require.Equal(t, string(gwSecret.Data["tls.crt"]), newTLSCertValue)
	require.Equal(t, string(gwSecret.Data["tls.key"]), newTLSKeyValue)
	require.Equal(t, string(gwSecret.Data["ca.crt"]), newCACertValue)
}
