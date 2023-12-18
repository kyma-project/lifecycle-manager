package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Keep Consistent After Deploy", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
		})

		moduleOperator := "template-operator-controller-manager"
		It("Then Module Resources are deployed on SKR cluster", func() {
			By("And Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
			By("And Module Operator Deployment is ready")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleOperator, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("Then synced module resources remain consistent with the same resourceVersion", func() {
			manifest, err := GetManifest(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name)
			Expect(err).Should(Succeed())
			for _, resource := range manifest.Status.Synced {
				objectKey := client.ObjectKey{Name: resource.Name, Namespace: resource.Namespace}
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				resourceInCluster, err := GetCR(ctx, runtimeClient,
					objectKey,
					gvk)
				Expect(err).Should(Succeed())
				Consistently(IsResourceVersionSame).
					WithContext(ctx).
					WithArguments(runtimeClient, objectKey, gvk,
						resourceInCluster.GetResourceVersion()).Should(BeTrue())
			}
		})

		It("When Stop Module Operator", func() {
			Eventually(SetSkipLabelToManifest).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name, true).
				Should(Succeed())

			Eventually(StopDeployment).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleOperator, TestModuleResourceNamespace).
				Should(Succeed())

			Eventually(SetSkipLabelToManifest).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name, false).
				Should(Succeed())
		})

		It("Then Module Operator has been reset", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleOperator, TestModuleResourceNamespace).
				Should(Succeed())
		})
	})
})
