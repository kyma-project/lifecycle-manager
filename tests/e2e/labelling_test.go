package e2e_test

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	managedSkrResources = map[types.NamespacedName]schema.GroupVersionKind{
		{Name: "default", Namespace: RemoteNamespace}:     v1beta2.GroupVersion.WithKind("Kyma"),
		{Name: "skr-webhook", Namespace: RemoteNamespace}: apiappsv1.SchemeGroupVersion.WithKind("Deployment"),
		{Name: "skr-webhook", Namespace: RemoteNamespace}: admissionregistrationv1.SchemeGroupVersion.WithKind(
			"ValidatingWebhookConfiguration",
		),
		{Name: "skr-webhook-tls", Namespace: RemoteNamespace}: apicorev1.SchemeGroupVersion.WithKind("Secret"),
		{Name: RemoteNamespace, Namespace: ""}:                apicorev1.SchemeGroupVersion.WithKind("Namespace"),
	}
	globalAccountIDLabelValue = "dummy-global-account"
	subAccountIDLabelValue    = "dummy-sub-account"
)

var _ = Describe("Labelling SKR resources", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	setBTPRelatedLabels(kyma)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	module := NewTemplateOperator(v1beta2.DefaultChannel)

	Context("Given SKR Cluster", func() {
		It("When SKR cluster is setup", func() {
			By("Then SKR Kyma CR is labelled with watched-by label")
			Eventually(HasExpectedLabel).
				WithContext(ctx).
				WithArguments(
					skrClient,
					types.NamespacedName{Name: "default", Namespace: RemoteNamespace},
					v1beta2.GroupVersion.WithKind("Kyma"),
					shared.WatchedByLabel,
					shared.WatchedByLabelValue).
				Should(Succeed())
			By("And managed SKR resources are labelled with managed-by label")
			for name, gvk := range managedSkrResources {
				Eventually(HasExpectedLabel).
					WithContext(ctx).
					WithArguments(
						skrClient,
						name,
						gvk,
						shared.ManagedBy,
						shared.ManagedByLabelValue).
					Should(Succeed())
			}
			By("And remote namespace is labelled with istio and warden labels")
			Eventually(EnsureNamespaceHasCorrectLabels).
				WithContext(ctx).
				WithArguments(skrClient, RemoteNamespace, map[string]string{
					"istio-injection": "enabled",
					"namespaces.warden.kyma-project.io/validate": "enabled",
				}).Should(Succeed())
		})
		It("Contains BTP related Labels", func() {
			By("Global account label being propagated")
			Eventually(HasExpectedLabel).
				WithContext(ctx).
				WithArguments(
					skrClient,
					types.NamespacedName{Name: "default", Namespace: RemoteNamespace},
					v1beta2.GroupVersion.WithKind("Kyma"),
					shared.GlobalAccountIDLabel, globalAccountIDLabelValue).
				Should(Succeed())
			By("Sub account label being propagated")
			Eventually(HasExpectedLabel).
				WithContext(ctx).
				WithArguments(
					skrClient,
					types.NamespacedName{Name: "default", Namespace: RemoteNamespace},
					v1beta2.GroupVersion.WithKind("Kyma"),
					shared.SubAccountIDLabel, subAccountIDLabelValue).
				Should(Succeed())
		})

		It("When Kyma Module is enabled in SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
				Should(Succeed())

			By("Then all manifest resources are labelled with managed-by label")
			manifest, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name)
			Expect(err).Should(Succeed())
			for _, resource := range manifest.Status.Synced {
				name := types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace}
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(HasExpectedLabel).
					WithContext(ctx).
					WithArguments(skrClient, name, gvk,
						shared.ManagedBy, shared.ManagedByLabelValue).Should(Succeed())
			}

			By("And default CR is labbelled with managed-by label")
			Eventually(CheckSampleCRHasExpectedLabel).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.ManagedBy,
					shared.ManagedByLabelValue).
				Should(Succeed())
		})
	})
})

func setBTPRelatedLabels(kyma *v1beta2.Kyma) {
	kyma.Labels[shared.GlobalAccountIDLabel] = globalAccountIDLabelValue
	kyma.Labels[shared.SubAccountIDLabel] = subAccountIDLabelValue
}
