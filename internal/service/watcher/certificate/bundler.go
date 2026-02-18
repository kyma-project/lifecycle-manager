package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

const (
	certificateBlockType = "CERTIFICATE"
)

var (
	ErrInvalidPEM         = errors.New("invalid PEM")
	ErrFailedToParseX509  = errors.New("failed to parse x509 certificate")
	ErrX509NotAfterIsZero = errors.New("x509 certificate NotAfter is zero")
)

type Bundler struct {
	parseX509Func func([]byte) (*x509.Certificate, error)
}

func NewBundler(opts ...func(*Bundler) *Bundler) *Bundler {
	bundler := &Bundler{
		parseX509Func: x509.ParseCertificate,
	}

	for _, opt := range opts {
		bundler = opt(bundler)
	}

	return bundler
}

// WithParseX509Function is a low level primitive that replaces the default x509.ParseCertificate function.
func WithParseX509Function(f func([]byte) (*x509.Certificate, error)) func(*Bundler) *Bundler {
	return func(b *Bundler) *Bundler {
		b.parseX509Func = f
		return b
	}
}

// Bundle adds the given cert to the bundle.
// It returns true if the cert was added, false if it was already present.
func (b Bundler) Bundle(bundle *[]byte, newCert []byte) (bool, error) {
	newCertPEM, _ := pem.Decode(newCert)
	if newCertPEM == nil {
		return false, fmt.Errorf("invalid newCert: %w", ErrInvalidPEM)
	}

	certsPEM, err := decodeCertsToPEM(*bundle)
	if err != nil {
		return false, fmt.Errorf("invalid bundle: %w", err)
	}

	if containsCert(certsPEM, newCertPEM) {
		return false, nil
	}

	certsPEM = prependCert(certsPEM, newCertPEM)

	*bundle = encodePEMToBytes(certsPEM)
	return true, nil
}

// DropExpiredCerts removes expired certs from the bundle.
// It returns true if any certs were removed, false otherwise.
func (b Bundler) DropExpiredCerts(bundle *[]byte) (bool, error) {
	certsPEM, err := decodeCertsToPEM(*bundle)
	if err != nil {
		return false, err
	}

	certsX509, err := b.decodePEMToX509(certsPEM)
	if err != nil {
		return false, err
	}

	unexpiredCertsX509, err := dropExpiredCerts(certsX509)
	if err != nil || len(unexpiredCertsX509) == len(certsX509) {
		return false, err
	}

	*bundle = encodePEMToBytes(encodeX509ToPEM(unexpiredCertsX509))

	return true, nil
}

func decodeCertsToPEM(bundle []byte) ([]*pem.Block, error) {
	var certs []*pem.Block
	rest := bundle
	for len(rest) > 0 {
		var cert *pem.Block
		cert, rest = pem.Decode(rest)
		if cert != nil {
			certs = append(certs, cert)
		} else {
			return nil, ErrInvalidPEM
		}
	}
	return certs, nil
}

func (b Bundler) decodePEMToX509(certsPEM []*pem.Block) ([]*x509.Certificate, error) {
	certsX509 := make([]*x509.Certificate, len(certsPEM))
	for i, certBlock := range certsPEM {
		cert, err := b.parseX509Func(certBlock.Bytes)
		if err != nil {
			return nil, errors.Join(ErrFailedToParseX509, err)
		}
		certsX509[i] = cert
	}
	return certsX509, nil
}

func encodeX509ToPEM(certsX509 []*x509.Certificate) []*pem.Block {
	certsPEM := make([]*pem.Block, len(certsX509))
	for i, cert := range certsX509 {
		certBlock := &pem.Block{
			Type:  certificateBlockType,
			Bytes: cert.Raw,
		}
		certsPEM[i] = certBlock
	}
	return certsPEM
}

func containsCert(certs []*pem.Block, newCert *pem.Block) bool {
	for _, cert := range certs {
		if string(cert.Bytes) == string(newCert.Bytes) {
			return true
		}
	}
	return false
}

func prependCert(certs []*pem.Block, newCert *pem.Block) []*pem.Block {
	newBundle := make([]*pem.Block, 0, 1+len(certs))
	newBundle = append(newBundle, newCert)
	newBundle = append(newBundle, certs...)
	return newBundle
}

func encodePEMToBytes(certs []*pem.Block) []byte {
	// Each PEM block is usually ~1-2KB; preallocate to reduce reallocations.
	// The exact size depends on cert length, so this is a best-effort estimate.
	const avgPEMBlockSizeBytes = 2 * 1024
	bundle := make([]byte, 0, len(certs)*avgPEMBlockSizeBytes)
	for _, cert := range certs {
		bundle = append(bundle, pem.EncodeToMemory(cert)...)
	}
	return bundle
}

func dropExpiredCerts(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	var validCerts []*x509.Certificate
	for _, cert := range certs {
		isExpired, err := expired(cert)
		if err != nil {
			return nil, err
		}

		if !isExpired {
			validCerts = append(validCerts, cert)
		}
	}
	return validCerts, nil
}

func expired(cert *x509.Certificate) (bool, error) {
	if cert.NotAfter.IsZero() {
		return false, ErrX509NotAfterIsZero
	}

	return cert.NotAfter.Before(time.Now()), nil
}
