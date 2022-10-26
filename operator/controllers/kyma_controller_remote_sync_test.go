package controllers_test

import (
	"encoding/json"
	"time"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	var kyma *v1alpha1.Kyma
	var skrModule *v1alpha1.Module
	skrModuleFromClient := &v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1alpha1.ChannelStable,
	}
	kyma = NewTestKyma("kyma-remote-sync")
	skrModule = &v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync",
		Channel:        v1alpha1.ChannelStable,
	}

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule)

	RegisterDefaultLifecycleForKyma(kyma)

	It("module template created", func() {
		template, err := test.ModuleTemplateFactory(*skrModuleFromClient, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
	})

	It("CR add from client should be synced in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma.GetName()), 30*time.Second, interval).Should(Succeed())
		remoteKyma, err := GetKyma(runtimeClient, kyma.GetName())
		Expect(err).ShouldNot(HaveOccurred())

		By("add skr-module-client to remoteKyma.spec.modules")
		remoteKyma.Spec.Modules = []v1alpha1.Module{
			*skrModuleFromClient,
		}
		Eventually(runtimeClient.Update(ctx, remoteKyma), timeout, interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(kyma.GetName(), skrModuleFromClient.Name),
			timeout, interval).Should(BeTrue())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-remote-module",
		Channel:        v1alpha1.ChannelStable,
	})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma.GetName()), 30*time.Second, interval).Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(kyma.GetName(), activeModule.Name), timeout, interval).Should(BeTrue())
		}

		By("No spec.module in remote Kyma")
		remoteKyma, err := GetKyma(runtimeClient, kyma.GetName())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(remoteKyma.Spec.Modules).To(BeEmpty())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplatesExist(runtimeClient, kyma), 30*time.Second, interval).Should(Succeed())
		Expect(remoteKyma.ContainsCondition(v1alpha1.ConditionTypeReady,
			v1alpha1.ConditionReasonModuleCatalogIsReady)).To(BeTrue())

		By("updating a module template in the remote cluster to simulate unwanted modification")
		Eventually(ModifyModuleTemplateSpecThroughLabels(runtimeClient, kyma,
			ocm.Labels{ocm.Label{Name: "test", Value: json.RawMessage(`{"foo":"bar"}`)}}), timeout, interval).Should(Succeed())

		By("verifying the discovered override and checking the resetted label")
		Eventually(ModuleTemplatesLabelsCountMatch(runtimeClient, kyma, 0), timeout, interval).Should(Succeed())
		Eventually(ModuleTemplatesLastSyncGenMatches(runtimeClient, kyma), timeout, interval).Should(BeTrue())
	})
})
