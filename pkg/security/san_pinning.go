package security

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	requestSizeLimit = 16000

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
func (v *RequestVerifier) Verify(request *http.Request) error {
	certificate, err := v.getCertificateFromHeader(request)
	if err != nil {
		return err
	}

	domain, err := v.getDomain(request)
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

	v.Log.Info(fmt.Sprintf("###### Request Header %v", xfccValue))
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
	v.Log.V(int(zap.DebugLevel)).Info(XFCCHeader,
		"certificate", certificate)

	return certificate, nil
}

// getDomain fetches the KymaCR, mentioned in the requests body, and returns the value of the SKR-Domain annotation.
func (v *RequestVerifier) getDomain(request *http.Request) (string, error) {
	limitedReader := &io.LimitedReader{R: request.Body, N: requestSizeLimit}
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	defer request.Body.Close()
	request.Body = io.NopCloser(bytes.NewBuffer(body))

	watcherEvent := &types.WatchEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return "", err
	}
	var kymaCR v1alpha1.Kyma
	if err := v.Client.Get(request.Context(), watcherEvent.Owner, &kymaCR); err != nil {
		return "", err
	}
	domain, ok := kymaCR.Annotations[shootDomainKey]
	if !ok {
		return "", AnnotationMissingError{
			KymaCR:     watcherEvent.Owner.String(),
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
		v.Log.V(int(zap.DebugLevel)).Info("Received request verified")
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
