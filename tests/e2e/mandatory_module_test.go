package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mandatory Module Installation and Deletion", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("Then mandatory module is installed on the SKR cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion,
					TestModuleResourceNamespace).
				Should(Succeed())
			By("And the SKR Module Default CR is in a \"Ready\" State", func() {
				Eventually(CheckSampleCRIsInState).
					WithContext(ctx).
					WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.StateReady).
					Should(Succeed())
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
			By("And the Mandatory ModuleReleaseMeta is configured correctly", func() {
				Eventually(MandatoryModuleReleaseMetaHasVersion).
					WithContext(ctx).
					WithArguments(kcpClient, "template-operator", "1.1.0-smoke-test").
					Should(Succeed())
			})

			By("And the mandatory ModuleTemplate is not synchronised to the SKR cluster", func() {
				Consistently(CheckIfExists).
					WithContext(ctx).
					WithArguments("template-operator", RemoteNamespace,
						shared.OperatorGroup, "v1beta2", string(shared.ModuleTemplateKind), skrClient).
					Should(Not(Succeed()))
			})
			By("And the mandatory ModuleReleaseMeta is not synchronised to the SKR cluster", func() {
				Consistently(CheckIfExists).
					WithContext(ctx).
					WithArguments("template-operator", RemoteNamespace,
						shared.OperatorGroup, "v1beta2", string(shared.ModuleReleaseMetaKind), skrClient).
					Should(Not(Succeed()))
			})

			By("And the mandatory module manifest is installed with the correct version", func() {
				Consistently(MandatoryModuleManifestExistWithCorrectVersion).
					WithContext(ctx).
					WithArguments(kcpClient, "template-operator", "1.1.0-smoke-test").
					Should(Succeed())
			})

			By("And the mandatory module manifest does not contain the operator.kyma-project.io/channel label")
			Eventually(MandatoryModuleManifestContainsExpectedLabel).
				WithContext(ctx).
				WithArguments(kcpClient, "template-operator", "operator.kyma-project.io/channel", "").
				Should(Equal(ErrManifestNotContainLabelKey))
		})

		It("When the mandatory Manifest is labelled to skip reconciliation", func() {
			Eventually(SetSkipLabelToMandatoryManifests).
				WithContext(ctx).
				WithArguments(kcpClient, true).
				Should(Succeed())

			By("And deleting the mandatory SKR Default CR", func() {
				Eventually(DeleteCRWithGVK).
					WithContext(ctx).
					WithArguments(skrClient, TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
						"v1alpha1", string(templatev1alpha1.SampleKind)).
					Should(Succeed())
			})
		})
		It("Then mandatory SKR Module Default CR is not recreated", func() {
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					"v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrNotFound))
		})

		It("When deleting the mandatory SKR Module Manager Deployment", func() {
			err := DeleteCRWithGVK(ctx, skrClient, ModuleDeploymentNameInOlderVersion,
				TestModuleResourceNamespace, "apps", "v1", "Deployment")
			Expect(err).ToNot(HaveOccurred())
		})
		It("Then Module Manager Deployment is not recreated on the SKR cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion,
					TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When the mandatory Manifest skip reconciliation label is removed", func() {
			Eventually(SetSkipLabelToMandatoryManifests).
				WithContext(ctx).
				WithArguments(kcpClient, false).
				Should(Succeed())
		})
		It("Then mandatory SKR Module Default CR is recreated", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					templatev1alpha1.GroupVersion.Group, "v1alpha1", string(templatev1alpha1.SampleKind),
					skrClient).
				Should(Succeed())

			By("And mandatory SKR Module Deployment is recreated", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleDeploymentNameInOlderVersion,
						TestModuleResourceNamespace).
					Should(Succeed())
			})

			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When new version of ModuleTemplate is applied", func() {
			cmd := exec.Command("kubectl", "apply", "-f",
				"mandatory_template_v2.yaml")
			_, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			err = SetMandatoryModuleReleaseMetaVersion(ctx, kcpClient, "template-operator", ControlPlaneNamespace,
				"2.4.1-smoke-test")
			Expect(err).NotTo(HaveOccurred())
		})
		It("Then Kyma mandatory Module is updated on SKR Cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion,
					TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment is removed", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleDeploymentNameInOlderVersion,
						TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})

			By("And the mandatory module manifest is installed with the correct version", func() {
				Eventually(MandatoryModuleManifestExistWithCorrectVersion).
					WithContext(ctx).
					WithArguments(kcpClient, "template-operator", "2.4.1-smoke-test").
					Should(Succeed())
			})
		})

		It("When the mandatory ModuleReleaseMeta is deleted", func() {
			Eventually(DeleteModuleReleaseMeta).
				WithContext(ctx).
				WithArguments("template-operator", ControlPlaneNamespace, kcpClient).
				Should(Succeed())
		})
		It("Then mandatory SKR module is removed", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion,
					TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And the mandatory SKR Module Default CR is removed", func() {
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments(TestModuleCRName, RemoteNamespace,
						templatev1alpha1.GroupVersion.Group, "v1alpha1", string(templatev1alpha1.SampleKind),
						skrClient).
					Should(Equal(ErrNotFound))
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
			By("And the mandatory module manifest is not present in the KCP cluster", func() {
				Eventually(MandatoryModuleManifestExistWithCorrectVersion).
					WithContext(ctx).
					WithArguments(kcpClient, "template-operator", "2.4.1-smoke-test").
					Should(Equal(ErrManifestNotFound))
			})
		})
	})
})
