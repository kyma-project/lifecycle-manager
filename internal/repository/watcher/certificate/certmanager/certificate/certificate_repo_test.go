package certificate_test

import (
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	certmanagercertificate "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/certmanager/certificate" //nolint:revive // not for import
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/errors"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	issuerName         = random.Name()
	certName           = random.Name()
	certNamespace      = random.Name()
	certCommonNameName = random.Name()
	certDNSNames       = []string{
		random.Name(),
		random.Name(),
	}
	certDuration    = 24 * time.Hour
	certRenewBefore = 12 * time.Hour
	certKeySize     = 4096
)

func Test_GetCacheObjects(t *testing.T) {
	objects := certmanagercertificate.GetCacheObjects()
	require.Len(t, objects, 1)
	assert.IsType(t, &certmanagerv1.Certificate{}, objects[0])
}

func TestNew_Namespace_Error(t *testing.T) {
	certClient, err := certmanagercertificate.NewRepository(
		nil,
		issuerName,
		config.CertificateValues{},
	)

	require.ErrorIs(t, err, certerror.ErrCertRepoConfigNamespace)
	assert.Nil(t, certClient)
}
