package certificate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
)

func Test_GetCertificateLabels(t *testing.T) {
	labels := certificate.GetCertificateLabels()

	assert.Len(t, labels, 2)
	assert.Equal(t, shared.CertManager, labels[shared.PurposeLabel])
	assert.Equal(t, shared.OperatorName, labels[shared.ManagedBy])
}
