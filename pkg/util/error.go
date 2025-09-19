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

var ErrClientUnauthorized = errors.New("ServerSideApply is unauthorized")

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
	return errors.Is(err,
		syscall.ECONNREFUSED) || apierrors.IsUnauthorized(err) || isNoSuchHostError(err) || errors.Is(err,
		ErrClientUnauthorized)
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
