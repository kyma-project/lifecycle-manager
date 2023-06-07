package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("Kyma with managed fields", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		_, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})
	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ExpectKymaManagerField, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), v1beta2.OperatorName).
			Should(BeTrue())
	})

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})
