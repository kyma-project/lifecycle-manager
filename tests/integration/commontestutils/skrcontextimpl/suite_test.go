package skrcontextimpl

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestSkrContextImpl provides a Ginkgo test hook so that ginkgo-specific
// flags (e.g. -ginkgo.flake-attempts) passed by the Makefile are recognized.
// It does not define any specs itself; existing standard library tests in this
// package will continue to run alongside an empty Ginkgo suite.
func TestSkrContextImpl(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SkrContextImpl Suite")
}
