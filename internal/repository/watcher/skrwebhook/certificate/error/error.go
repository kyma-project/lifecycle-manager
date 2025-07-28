package error

import "errors"

var (
	ErrNoRenewalTime           = errors.New("no renewal time set for certificate")
	ErrNoNotBefore             = errors.New("notBefore not found")
	ErrNoNotAfter              = errors.New("notAfter not found")
	ErrCertRepoConfigNamespace = errors.New("certificates namespace is not set")
)
