package util

import (
	"errors"
	"fmt"
	"net"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
)

const (
	NoSuchHostMsg = "no such host"
)

func IsNoSuchHost(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Err == NoSuchHostMsg
	}
	fmt.Println("----------------------------------------")
	fmt.Printf("this error is not a net.DNSError but: %T", err)
	return false
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
