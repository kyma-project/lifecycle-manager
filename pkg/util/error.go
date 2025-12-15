package util

import (
	"errors"
	"net"
	"strings"
	"syscall"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
)

const (
	msgTLSCertificateExpired = "expired certificate" // stdlib: crypto/tls/alert.go
	msgUnauthorized          = ": Unauthorized"      // message we observe when SKR cert is not valid anymore
)

var (
	ErrClientUnauthorized   = errors.New("ServerSideApply is unauthorized")
	ErrClientTLSCertExpired = errors.New("SKR access secret certificate is expired")
)

func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if machineryruntime.IsNotRegisteredError(err) ||
		meta.IsNoMatchError(err) ||
		apierrors.IsNotFound(err) {
		return true
	}

	// Introduced in controller-runtime v0.15.0, which makes a simple
	// `k8serrors.IsNotFound(err)` not work any more.
	groupErr := &discovery.ErrGroupDiscoveryFailed{}
	if errors.As(err, &groupErr) {
		for _, err := range groupErr.Groups {
			if apierrors.IsNotFound(err) {
				return true
			}
		}
	}

	// Fallback
	for _, msg := range []string{
		"failed to get restmapping",
		"could not find the requested resource",
	} {
		if strings.Contains(err.Error(), msg) {
			return true
		}
	}
	return false
}

func IsConnectionRelatedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.ECONNREFUSED) ||
		apierrors.IsUnauthorized(err) ||
		apierrors.IsForbidden(err) ||
		isNoSuchHostError(err) ||
		errors.Is(err, ErrClientTLSCertExpired) ||
		errors.Is(err, ErrClientUnauthorized) ||
		isRawCertRelatedError(err) // last resort check
}

func isNoSuchHostError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.IsNotFound
	}
	return false
}

// NestedErrorMessage returns the error message of the wrapped error, if it exists.
// Otherwise, it returns an empty string.
func NestedErrorMessage(err error) string {
	res := ""

	if err == nil {
		return res
	}
	if uwErr := errors.Unwrap(err); uwErr != nil {
		res = uwErr.Error()
	}

	return res
}

// isRawCertRelatedError checks if the error message contains common
// substrings related to certificate issues, such as expired or invalid certificates.
// These errors may have types that are not easily identifiable (or not exported),
// so we can't use errors.Is() to check for them.
// In our code we try to detect these errors and replace them with a well-known error types like
// ErrClientTLSCertExpired and ErrClientUnauthorized.
// However, certain code paths may still return "raw" underlying errors and so
// we need to handle these as well.
func isRawCertRelatedError(err error) bool {
	return IsUnauthorizedError(err.Error()) || IsTLSCertExpiredError(err.Error())
}

func IsUnauthorizedError(errMessage string) bool {
	return strings.HasSuffix(strings.TrimRight(errMessage, " \n"), msgUnauthorized)
}

func IsTLSCertExpiredError(errMessage string) bool {
	return strings.HasSuffix(strings.TrimRight(errMessage, " \n"), msgTLSCertificateExpired)
}
