package certificate_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/errors"
	gcmcertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/gcm/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var rsaKeyAlgorithm = gcertv1alpha1.RSAKeyAlgorithm

var (
	issuerName                                   = random.Name()
	issuerNamespace                              = random.Name()
	certName                                     = random.Name()
	certNamespace                                = random.Name()
	certCommonName                               = random.Name()
	certDNSNames                                 = []string{random.Name(), random.Name()}
	certDuration                                 = 24 * time.Hour
	certRenewBefore                              = 12 * time.Hour
	certKeySize     gcertv1alpha1.PrivateKeySize = 4096
)

func Test_GetCacheObjects(t *testing.T) {
	objects := gcmcertificate.GetCacheObjects()
	require.Len(t, objects, 1)
	assert.IsType(t, &gcertv1alpha1.Certificate{}, objects[0])
}

func TestNew_MaxIntKeySize_Error(t *testing.T) {
	invalidKeySize := math.MaxInt32 + 1

	certClient, err := gcmcertificate.NewRepository(
		nil,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			KeySize: invalidKeySize,
		},
	)

	require.ErrorIs(t, err, gcmcertificate.ErrGCMRepoConfigKeySizeOutOfRange)
	assert.Nil(t, certClient)
}

func TestNew_MinIntKeySize_Error(t *testing.T) {
	invalidKeySize := math.MinInt32 - 1

	certClient, err := gcmcertificate.NewRepository(
		nil,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			KeySize: invalidKeySize,
		},
	)

	require.ErrorIs(t, err, gcmcertificate.ErrGCMRepoConfigKeySizeOutOfRange)
	assert.Nil(t, certClient)
}

func TestNew_Namespace_Error(t *testing.T) {
	certClient, err := gcmcertificate.NewRepository(
		nil,
		issuerName,
		issuerNamespace,
		config.CertificateValues{
			KeySize: int(certKeySize),
		},
	)

	require.ErrorIs(t, err, certerror.ErrCertRepoConfigNamespace)
	assert.Nil(t, certClient)
}

func stringPtr(s string) *string {
	return &s
}
