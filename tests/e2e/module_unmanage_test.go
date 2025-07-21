package e2e_test

import (
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unmanaging Kyma Module", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	module := NewTemplateOperator(v1beta2.DefaultChannel)

	Context("Given SKR Cluster", func() {
		var manifestResources []shared.Resource
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module Operator is deployed on SKR cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace, "apps", "v1",
					"Deployment", skrClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			By("And all manifest resources and default CR have managed-by label")
			manifest, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name)
			Expect(err).Should(Succeed())
			manifestResources = manifest.Status.Synced
			for _, resource := range manifestResources {
				objectKey := client.ObjectKey{Name: resource.Name, Namespace: resource.Namespace}
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(HasExpectedLabel).
					WithContext(ctx).
					WithArguments(skrClient, objectKey, gvk,
						shared.ManagedBy, shared.ManagedByLabelValue).Should(Succeed())

			}
			Eventually(CheckSampleCRHasExpectedLabel).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.ManagedBy,
					shared.ManagedByLabelValue).
				Should(Succeed())
		})

		It("When Kyma Module is unmanaged", func() {
			Eventually(SetModuleManaged).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name, false).
				Should(Succeed())

			By("And Manifest CR is removed")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("Then Module CR on SKR Cluster is not removed")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind), skrClient).
				Should(Succeed())

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", skrClient).
				Should(Succeed())

			By("And all manifest resources and default CR no longer have managed-by labels")
			for _, resource := range manifestResources {
				objectKey := client.ObjectKey{Name: resource.Name, Namespace: resource.Namespace}
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(HasExpectedLabel).
					WithContext(ctx).
					WithArguments(skrClient, objectKey, gvk,
						shared.ManagedBy, shared.ManagedByLabelValue).Should(Equal(ErrLabelNotFound))
			}

			Eventually(CheckSampleCRHasExpectedLabel).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.ManagedBy,
					shared.ManagedByLabelValue).
				Should(Equal(ErrLabelNotExistOnCR))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And count of Kyma Module Metric in \"Unmanaged\" State is 1")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, shared.StateUnmanaged).
				Should(Equal(1))
		})

		It("When Kyma Module is set to managed again", func() {
			Eventually(SetModuleManaged).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name, true).
				Should(Succeed())
			By("Then Manifest is re-created", func() {
				Eventually(CheckManifestIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient, shared.StateReady).
					Should(Succeed())
				Eventually(ManifestNoDeletionTimeStampSet).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient).
					Should(Succeed())
			})
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			By("And count of Kyma Module Metric in \"Unmanaged\" State is 0")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, shared.StateUnmanaged).
				Should(Equal(0))
		})

		It("When Kyma Module is unmanaged again", func() {
			Eventually(SetModuleManaged).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name, false).
				Should(Succeed())

			By("Then Manifest CR is removed")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())

			By("Then Module CR on SKR Cluster is not removed")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind), skrClient).
				Should(Succeed())

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", skrClient).
				Should(Succeed())
		})

		It("When Module is enabled again with CustomResourcePolicy:Ignore", func() {
			module.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore

			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())

			By("Then Manifest is re-created", func() {
				Eventually(CheckManifestIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient, shared.StateReady).
					Should(Succeed())
				Eventually(ManifestNoDeletionTimeStampSet).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient).
					Should(Succeed())
			})

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Module is unmanaged", func() {
			Eventually(SetModuleManaged).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name, false).
				Should(Succeed())

			By("And Manifest CR is removed")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", skrClient).
				Should(Succeed())

			By("And all manifest resources no longer have managed-by labels")
			for _, resource := range manifestResources {
				objectKey := client.ObjectKey{Name: resource.Name, Namespace: resource.Namespace}
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(HasExpectedLabel).
					WithContext(ctx).
					WithArguments(skrClient, objectKey, gvk,
						shared.ManagedBy, shared.ManagedByLabelValue).Should(Equal(ErrLabelNotFound))
			}

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
