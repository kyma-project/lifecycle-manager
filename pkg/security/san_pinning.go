package security

import (
	"bytes"
	"context"
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

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	debugLogLevel = 2

	xfccHeader           = "X-Forwarded-Client-Cert"
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
	errHeaderMissing = fmt.Errorf("request does not contain '%s' header", xfccHeader)

	// Error templates.
	errXFCCParse          = "failed to parse PEM block into x509 certificate: %w"
	errDecodeURL          = "could not decode certificate URL format: %w"
	errAnnotationsMissing = "KymaCR '%s' does not have annotation `%s`"
)

type RequestVerifier struct {
	Client client.Client
	log    logr.Logger
}

func NewRequestVerifier(client client.Client) RequestVerifier {
	return RequestVerifier{
		Client: client,
		log:    ctrl.Log.WithName("listener"),
	}
}

// Verify verifies the given request by fetching the KymaCR given in the request payload
// and comparing the SAN(subject alternative name) of the certificate with the SKR-domain of the KymaCR.
// If the request can be verified 'nil' will be returned.
func (v RequestVerifier) Verify(request *http.Request) error {
	certificate, err := v.getCertificateFromHeader(request)
	if err != nil {
		return err
	}

	domain, err := v.getDomain(request)
	if err != nil {
		return err
	}

	if containsURI(certificate.URIs, domain) ||
		containsDNS(certificate.DNSNames, domain) ||
		containsIP(certificate.IPAddresses, domain) {
		v.log.V(debugLogLevel).Info("Received request verified")
		return nil
	}
	return errNotVerified
}

// getCertificateFromHeader extracts the XFCC header and pareses it into a valid x509 certificate.
func (v RequestVerifier) getCertificateFromHeader(r *http.Request) (*x509.Certificate, error) {
	// Fetch XFCC-Header data
	xfccValue, ok := r.Header[xfccHeader]
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
		return nil, fmt.Errorf(errDecodeURL, err) //nolint:goerr113
	}
	decodedValue = strings.Trim(decodedValue, "\"")

	// Decode PEM block and parse certificate
	block, _ := pem.Decode([]byte(decodedValue))
	if block == nil {
		return nil, errPemDecode
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf(errXFCCParse, err) //nolint:goerr113
	}
	v.log.V(debugLogLevel).Info("X-Forwarded-Client-Certificate",
		"certificate", certificate)

	return certificate, nil
}

// getDomain fetches the KymaCR, mentioned in the requests body, and returns the value of the SKR-Domain annotation.
func (v *RequestVerifier) getDomain(request *http.Request) (string, error) {
	body, err := io.ReadAll(request.Body)
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
	if err := v.Client.Get(context.TODO(), watcherEvent.Owner, &kymaCR); err != nil {
		return "", err
	}
	domain, ok := kymaCR.Annotations[shootDomainKey]
	if !ok {
		return "", fmt.Errorf(errAnnotationsMissing, watcherEvent.Owner.String(), shootDomainKey) //nolint:goerr113
	}
	return domain, nil
}

// containsDNS checks if given DNS is present in slice.
func containsDNS(s []string, dns string) bool {
	for _, v := range s {
		if v == dns {
			return true
		}
	}
	return false
}

// containsURI checks if given URI is present in slice.
func containsURI(s []*url.URL, uri string) bool {
	for _, v := range s {
		if v.String() == uri {
			return true
		}
	}
	return false
}

// containsIP checks if given IP is present in slice.
func containsIP(s []net.IP, ip string) bool {
	for _, v := range s {
		if v.String() == ip {
			return true
		}
	}
	return false
}
