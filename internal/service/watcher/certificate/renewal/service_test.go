package renewal_test

import (
	"context"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	gatewaySecretName = random.Name()
	certName          = random.Name()
)

type mockCertificateRepository struct {
	renewCalled      bool
	renewErr         error
	getValidityErr   error
	getValidityStart time.Time
	getValidityEnd   time.Time
	receivedCertName string
}

func (m *mockCertificateRepository) Renew(_ context.Context, certName string) error {
	m.renewCalled = true
	m.receivedCertName = certName

	if m.renewErr != nil {
		return m.renewErr
	}

	return nil
}

func (m *mockCertificateRepository) GetValidity(_ context.Context, certName string) (time.Time, time.Time, error) {
	m.receivedCertName = certName
	return m.getValidityStart, m.getValidityEnd, m.getValidityErr
}
