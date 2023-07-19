package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var _ = Describe("Manifest.Spec.Remote in KCP mode", Ordered, func() {
	kyma := NewTestKyma("kyma")

	module := v1beta2.Module{
		ControllerName: "manifest",
		Name:           NewUniqModuleName(),
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		_, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})
	registerControlPlaneLifecycleForKyma(kyma)

	It("expect Manifest.Spec.Remote=true", func() {
		Eventually(GetManifestSpecRemote, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, module).
			Should(BeTrue())
	})

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})
