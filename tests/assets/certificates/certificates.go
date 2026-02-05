package certificates

import (
	_ "embed"
)

//go:embed certificate-valid-1.pem
var Cert1 []byte

//go:embed certificate-valid-2.pem
var Cert2 []byte

//go:embed certificate-valid-3.pem
var Cert3 []byte

//go:embed certificate-expired.pem
var CertExpired []byte
