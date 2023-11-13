package controlplane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Kyma with managed fields in kcp mode", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.DisableLabelValue

	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ContainsKymaManagerField, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), v1beta2.OperatorName).
			Should(BeTrue())
	})
})
