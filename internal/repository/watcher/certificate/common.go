package certificate

import (
	"errors"

	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	DefaultOrganizationalUnit = "BTP Kyma Runtime"
	DefaultOrganization       = "SAP SE"
	DefaultLocality           = "Walldorf"
	DefaultProvince           = "Baden-Württemberg"
	DefaultCountry            = "DE"
)

var ErrNoRenewalTime = errors.New("no renewal time set for certificate")

// GetCertificateLabels returns purpose and managed-by labels.
func GetCertificateLabels() k8slabels.Set {
	return k8slabels.Set{
		shared.PurposeLabel: shared.CertManager,
		shared.ManagedBy:    shared.OperatorName,
	}
}
