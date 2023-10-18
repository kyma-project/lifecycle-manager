package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma with managed fields in kcp mode", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.DisableLabelValue

	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ContainsExpectKymaManagerField, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), v1beta2.OperatorName).
			Should(BeTrue())
	})
})
