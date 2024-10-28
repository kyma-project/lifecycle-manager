package e2e_test

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("ModuleReleaseMeta Watch Trigger", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	moduleReleaseMetaNamespace := "kcp-system"
	moduleReleaseMetaName := "template-operator"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {

		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When ModuleReleaseMeta channels get updaed with invalid version", func() {
			Eventually(UpdateAllModuleReleaseMetaChannelVersions).
				WithContext(ctx).
				WithArguments(kcpClient, moduleReleaseMetaNamespace, moduleReleaseMetaName, "1.2.3").
				Should(Succeed())
		})
		It("Then KCP Kyma CR should be requeued and gets into \"Error\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
		})

	})
})

func UpdateAllModuleReleaseMetaChannelVersions(ctx context.Context, client client.Client, namespace, name, version string) error {

	meta := &v1beta2.ModuleReleaseMeta{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, meta); err != nil {
		return err
	}
	for i := range meta.Spec.Channels {
		meta.Spec.Channels[i].Version = version
	}
	if err := client.Update(ctx, meta); err != nil {
		return err
	}
	return nil
}
