package certificate

import (
	"errors"
	"time"

	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var ErrNoRenewalTime = errors.New("no renewal time set for certificate")

// CertificateConfig contains the configuration for the certificate.
// It is agnostic of the actual certiticate manager implementation.
type CertificateConfig struct {
	Duration    time.Duration
	RenewBefore time.Duration
	KeySize     int
}

// CertificateLabels are the labels that are added to the certificate.
var CertificateLabels = k8slabels.Set{
	shared.PurposeLabel: shared.CertManager,
	shared.ManagedBy:    shared.OperatorName,
}
