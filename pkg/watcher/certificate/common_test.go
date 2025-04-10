package certificate_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/stretchr/testify/assert"
)

func Test_GetCertificateLabels(t *testing.T) {
	labels := certificate.GetCertificateLabels()

	assert.Len(t, labels, 2)
	assert.Equal(t, shared.CertManager, labels[shared.PurposeLabel])
	assert.Equal(t, shared.OperatorName, labels[shared.ManagedBy])
}
