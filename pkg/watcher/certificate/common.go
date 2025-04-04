package certificate

import "time"

// CertificateConfig contains the configuration for the certificate.
// It is agnostic of the actual certiticate manager implementation.
type CertificateConfig struct {
	Duration    time.Duration
	RenewBefore time.Duration
	KeySize     int
}
