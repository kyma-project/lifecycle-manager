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
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const (
	XFCCHeader     = "X-Forwarded-Client-Cert"
	certificateKey = "Cert="
	shootDomainKey = "skr-domain"
	limit32KiB     = 32 * 1024
	limitSANValues = 100
)

var (

	// Static errors.
	errNotVerified        = errors.New("SAN from certificate does not match domain specified in KymaCR")
	errPemDecode          = errors.New("failed to decode PEM block")
	errEmptyCert          = errors.New("empty certificate")
	errHeaderValueTooLong = errors.New(XFCCHeader + " header value too long (over 32KiB)")
	errTooManySANValues   = errors.New("certificate contains too many SAN values (more than 100)")
	errHeaderMissing      = fmt.Errorf("request does not contain '%s' header", XFCCHeader)
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

	ok, err := v.VerifySAN(certificate, domain)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}
	return errNotVerified
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

// getCertificateFromHeader extracts the XFCC header and pareses it into a valid x509 certificate.
func (v *RequestVerifier) getCertificateFromHeader(r *http.Request) (*x509.Certificate, error) {
	// Fetch XFCC-Header data
	xfccValues, ok := r.Header[XFCCHeader]
	if !ok {
		return nil, errHeaderMissing
	}

	xfccVal := xfccValues[0]

	// Limit the length of the data (prevent resource exhaustion attack)
	if len(xfccVal) > limit32KiB {
		return nil, errHeaderValueTooLong
	}

	// Extract raw certificate from the first header value
	cert := getCertTokenFromXFCCHeader(xfccVal)
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
		return "", fmt.Errorf("failed to get Kyma CR: %w", err)
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

// getCertTokenFromXFCCHeader returns the first certificate embedded in the XFFC Header,
// if exists. Otherwise an empty string is returned.
func getCertTokenFromXFCCHeader(hVal string) string {
	certStartIdx := strings.Index(hVal, certificateKey)
	if certStartIdx >= 0 {
		tokenWithCert := hVal[(certStartIdx + len(certificateKey)):]
		// we shouldn't have "," here but it's safer to add it anyway
		certEndIdx := strings.IndexAny(tokenWithCert, ";,")
		if certEndIdx == -1 {
			// no suffix, the entire token is the cert value
			return tokenWithCert
		}

		// there's some data after the cert value, return just the cert part
		return tokenWithCert[:certEndIdx]
	}
	return ""
}
