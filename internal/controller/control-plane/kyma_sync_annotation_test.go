package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma sync default module list into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma")
	moduleInKCP := NewTestModule("in-kcp", v1beta2.DefaultChannel)
	kyma.Spec.Modules = []v1beta2.Module{{Name: moduleInKCP.Name, Channel: moduleInKCP.Channel}}

	remoteKyma := &v1beta2.Kyma{}
	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controller.DefaultRemoteSyncNamespace

	registerControlPlaneLifecycleForKyma(kyma)

	It("Kyma CR default module list should be copied to remote Kyma", func() {
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace).
			Should(Succeed())

		By("Remote Kyma contains default module")
		Eventually(containsModuleInSpec, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})

	It("Delete default module from remote Kyma", func() {
		By("Delete default module from remote Kyma")
		Eventually(removeModuleFromKyma, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being deleted")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(MatchError(ErrNotFound))
	})

	It("Default module list should be recreated if remote Kyma gets deleted", func() {
		By("Delete remote Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteKyma).Should(Succeed())
		By("Remote Kyma contains default module")
		Eventually(containsModuleInSpec, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.Name, remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("KCP Manifest is being created")
		Eventually(ManifestExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInKCP.Name).
			Should(Succeed())
	})

})
