package config

import "time"

// CertificateValues contains the configuration for the certificates the repositories will create.
// It is agnostic of the actual certificate manager implementation.
type CertificateValues struct {
	Duration    time.Duration
	RenewBefore time.Duration
	KeySize     int
}
