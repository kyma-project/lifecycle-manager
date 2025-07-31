package config

import "time"

// CertificateValues contains the configuration for the certificates the repository implementations will create.
type CertificateValues struct {
	Namespace   string
	Duration    time.Duration
	RenewBefore time.Duration
	KeySize     int
}
