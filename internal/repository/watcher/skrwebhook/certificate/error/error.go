package error

import "errors"

var ErrNoRenewalTime = errors.New("no renewal time set for certificate")
