package certificates

import (
	_ "embed"
)

//go:embed certificate-valid-1.pem
var Cert1 []byte // valid until Feb  2 10:17:07 2036 GMT

//go:embed certificate-valid-2.pem
var Cert2 []byte // valid until Feb  2 10:18:00 2036 GMT

//go:embed certificate-valid-3.pem
var Cert3 []byte // valid until Feb  2 12:00:19 2036 GMT

//go:embed certificate-expired.pem
var CertExpired []byte // expired on Feb  5 10:24:41 2026 GMT
