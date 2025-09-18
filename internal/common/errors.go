package common

import "errors"

var (
	ErrUnsupportedCertificateManagementSystem = errors.New("unsupported certificate management system")
	ErrNoOCIRegistryHostAndCredSecret         = errors.New(
		"the flags --oci-registry-host and --oci-registry-cred-secret cannot be both empty",
	)
	ErrBothOCIRegistryHostAndCredSecretProvided = errors.New(
		"only one of --oci-registry-host or --oci-registry-cred-secret should be provided",
	)
)
