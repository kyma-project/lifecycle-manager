package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Module Status Decoupling With StatefulSet", Ordered, func() {
	RunModuleStatusDecouplingTest(StatefulSetKind)
})
