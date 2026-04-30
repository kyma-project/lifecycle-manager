package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Restricted Modules - Normal Module Installation", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given KLM flag for restricted modules is set", func() {
		It("Then normal modules can be enabled as usual", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewTemplateOperator(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("And the Module CR has been installed on the SKR cluster")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And the Module Operator Deployment is ready on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the module is disabled and MRM gets a kymaSelector", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And the MRM is patched with a kymaSelector for a non-matching global account ID")
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, TestModuleName, ControlPlaneNamespace, &apimetav1.LabelSelector{
					MatchExpressions: []apimetav1.LabelSelectorRequirement{
						{
							Key:      "kyma-project.io/global-account-id",
							Operator: apimetav1.LabelSelectorOpIn,
							Values:   []string{"any-global-account-id"},
						},
					},
				}).
				Should(Succeed())

			By("And the module is enabled again on the SKR cluster")
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace,
					NewTemplateOperator(v1beta2.DefaultChannel)).
				Should(Succeed())

			By("Then the module is in error state on the KCP cluster")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, ControlPlaneNamespace, TestModuleName, shared.StateError).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Error\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateError).
				Should(Succeed())

			By("And the SKR Kyma status contains the expected error message for the module")
			Eventually(ModuleMessageInKymaStatusIsCorrect).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, TestModuleName,
					"module template not allowed: module has kymaSelector but is not in restricted modules list").
				Should(Succeed())
		})
	})

})
