package errors

import "errors"

var (
	ErrNoRenewalTime           = errors.New("no renewal time set for certificate")
	ErrNoNotBefore             = errors.New("notBefore not found")
	ErrNoNotAfter              = errors.New("notAfter not found")
	ErrCertRepoConfigNamespace = errors.New("repository needs to be initialized with a namespace for certificates")
)
