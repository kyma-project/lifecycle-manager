package common

import "errors"

var (
	ErrTypeAssert           = errors.New("type assertion failed")
	ErrAccessSecretNotFound = errors.New("access secret not found")
)
