package controlplane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Manifest.Spec.Remote in KCP mode", Ordered, func() {
	kyma := NewTestKyma("kyma")
	kyma.Labels[shared.SyncLabel] = v1beta2.DisableLabelValue

	module := NewTestModule("module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	registerControlPlaneLifecycleForKyma(kyma)

	It("expect Manifest.Spec.Remote=true", func() {
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(BeTrue())
	})
})
