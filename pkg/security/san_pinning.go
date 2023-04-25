package security

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const (
	XFCCHeader           = "X-Forwarded-Client-Cert"
	headerValueSeparator = ";"
	keyValueSeparator    = "="
	certificateKey       = "Cert="

	shootDomainKey = "skr-domain"
)

var (

	// Static errors.
	errNotVerified   = errors.New("SAN from certificate does not match domain specified in KymaCR")
	errPemDecode     = errors.New("failed to decode PEM block")
	errEmptyCert     = errors.New("empty certificate")
	errHeaderMissing = fmt.Errorf("request does not contain '%s' header", XFCCHeader)
)

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

// Verify verifies the given request by fetching the KymaCR given in the request payload
// and comparing the SAN(subject alternative name) of the certificate with the SKR-domain of the KymaCR.
// If the request can be verified 'nil' will be returned.
func (v *RequestVerifier) Verify(request *http.Request, watcherEvtObject *types.WatchEvent) error {
	certificate, err := v.getCertificateFromHeader(request)
	if err != nil {
		return err
	}

	domain, err := v.getDomain(request, watcherEvtObject)
	if err != nil {
		return err
	}

	if v.VerifySAN(certificate, domain) {
		return nil
	}
	return errNotVerified
}

// getCertificateFromHeader extracts the XFCC header and pareses it into a valid x509 certificate.
func (v *RequestVerifier) getCertificateFromHeader(r *http.Request) (*x509.Certificate, error) {
	// Fetch XFCC-Header data
	xfccValue, ok := r.Header[XFCCHeader]
	if !ok {
		return nil, errHeaderMissing
	}
	xfccData := strings.Split(xfccValue[0], headerValueSeparator)

	// Extract raw certificate
	var cert string
	for _, keyValuePair := range xfccData {
		if strings.Contains(keyValuePair, certificateKey) {
			cert = strings.Split(keyValuePair, keyValueSeparator)[1]
			break
		}
	}
	if cert == "" {
		return nil, errEmptyCert
	}

	// Decode URL-format
	decodedValue, err := url.QueryUnescape(cert)
	if err != nil {
		return nil, fmt.Errorf("could not decode certificate URL format: %w", err)
	}
	decodedValue = strings.Trim(decodedValue, "\"")

	// Decode PEM block and parse certificate
	block, _ := pem.Decode([]byte(decodedValue))
	if block == nil {
		return nil, errPemDecode
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PEM block into x509 certificate: %w", err)
	}

	return certificate, nil
}

// getDomain fetches the KymaCR, mentioned in the requests body, and returns the value of the SKR-Domain annotation.
func (v *RequestVerifier) getDomain(request *http.Request, watcherEvtObject *types.WatchEvent) (string, error) {
	var kymaCR v1beta2.Kyma
	if err := v.Client.Get(request.Context(), watcherEvtObject.Owner, &kymaCR); err != nil {
		return "", err
	}
	domain, ok := kymaCR.Annotations[shootDomainKey]
	if !ok {
		return "", AnnotationMissingError{
			KymaCR:     watcherEvtObject.Owner.String(),
			Annotation: shootDomainKey,
		}
	}
	return domain, nil
}

// VerifySAN checks if given domain exists in the SAN information of the given certificate.
func (v *RequestVerifier) VerifySAN(certificate *x509.Certificate, kymaDomain string) bool {
	if contains(certificate.URIs, kymaDomain) ||
		contains(certificate.DNSNames, kymaDomain) ||
		contains(certificate.IPAddresses, kymaDomain) {
		v.Log.V(log.DebugLevel).Info("Received request verified")
		return true
	}
	return false
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
