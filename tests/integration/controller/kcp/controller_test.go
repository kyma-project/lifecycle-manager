package kcp_test

import (
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma with managed fields in kcp mode", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")

	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ContainsKymaManagerField, Timeout, Interval).
			WithArguments(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), fieldowners.LegacyLifecycleManager).
			Should(BeTrue())
	})
})
