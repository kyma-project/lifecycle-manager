package certmanager_test

import (
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/certmanager"
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
	objects := certmanager.GetCacheObjects()
	require.Len(t, objects, 1)
	assert.IsType(t, &certmanagerv1.Certificate{}, objects[0])
}
