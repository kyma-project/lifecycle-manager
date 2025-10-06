package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Module Status Decoupling With Deployment", Ordered, func() {
	RunModuleStatusDecouplingTest(DeploymentKind)
})
