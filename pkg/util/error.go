package util

import (
	"errors"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
)

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if runtime.IsNotRegisteredError(err) ||
		meta.IsNoMatchError(err) ||
		k8serrors.IsNotFound(err) {
		return true
	}

	// Introduced in controller-runtime v0.15.0, which makes a simple
	// `k8serrors.IsNotFound(err)` not work any more.
	groupErr := &discovery.ErrGroupDiscoveryFailed{}
	if errors.As(err, &groupErr) {
		for _, err := range groupErr.Groups {
			if k8serrors.IsNotFound(err) {
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

func IsConnectionRefused(err error) bool {
	if err == nil {
		return false
	}

	for _, msg := range []string{
		"connection refused",
		"no such host",
		"connection reset by peer",
	} {
		if strings.Contains(err.Error(), msg) {
			return true
		}
	}

	return false
}
