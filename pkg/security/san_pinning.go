package security

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	limitSANValues = 100
)

var errTooManySANValues = errors.New("certificate contains too many SAN values (more than 100)")

type RequestVerifier struct {
	Client client.Client
	Log    logr.Logger
}

func NewRequestVerifier(client client.Client) *RequestVerifier {
	return &RequestVerifier{
		Client: client,
		Log:    ctrl.Log.WithName("request–verifier"),
	}
}

// VerifySAN checks if given domain exists in the SAN information of the given certificate.
func (v *RequestVerifier) VerifySAN(certificate *x509.Certificate, kymaDomain string) (bool, error) {
	uris := certificate.URIs
	dnsNames := certificate.DNSNames
	IPAddresses := certificate.IPAddresses

	if (len(uris) + len(dnsNames) + len(IPAddresses)) > limitSANValues {
		return false, errTooManySANValues
	}

	if contains(uris, kymaDomain) ||
		contains(dnsNames, kymaDomain) ||
		contains(IPAddresses, kymaDomain) {
		v.Log.V(log.DebugLevel).Info("Received request verified")
		return true, nil
	}

	return false, nil
}

// contains checks if given string is present in slice.
func contains[E net.IP | *url.URL | string](arr []E, s string) bool {
	for i := range arr {
		a := fmt.Sprintf("%s", arr[i])
		if a == s {
			return true
		}
	}
	return false
}

type AnnotationMissingError struct {
	KymaCR     string
	Annotation string
}

func (e AnnotationMissingError) Error() string {
	return fmt.Sprintf("KymaCR '%s' does not have annotation `%s`", e.KymaCR, e.Annotation)
}
