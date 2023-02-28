package controllers_test

import (
	"encoding/json"
	"time"

	ocmv1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	var kyma *v1beta1.Kyma
	var skrModule *v1beta1.Module
	skrModuleFromClient := &v1beta1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1beta1.DefaultChannel,
	}
	kyma = NewTestKyma("kyma-remote-sync")
	skrModule = &v1beta1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync",
		Channel:        v1beta1.DefaultChannel,
	}

	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
		Namespace:    metav1.NamespaceDefault,
		NoModuleCopy: true,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule)

	RegisterDefaultLifecycleForKyma(kyma)

	It("module template created", func() {
		template, err := ModuleTemplateFactory(*skrModuleFromClient, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
	})

	It("CR add from client should be synced in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace), 30*time.Second, Interval).
			Should(Succeed())

		By("add skr-module-client to remoteKyma.spec.modules")
		Eventually(UpdateRemoteModule(ctx, runtimeClient, kyma, []v1beta1.Module{
			*skrModuleFromClient,
		}), Timeout, Interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(ctx, kyma, *skrModuleFromClient), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
		Namespace:    "sync-namespace",
		NoModuleCopy: true,
	}

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           "skr-remote-module",
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace), 30*time.Second, Interval).
			Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("No spec.module in remote Kyma")
		remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(remoteKyma.Spec.Modules).To(BeEmpty())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplatesExist(runtimeClient, kyma, true), 30*time.Second, Interval).Should(Succeed())
		Eventually(func() {
			remoteKyma, err = GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(remoteKyma.ContainsCondition(v1beta1.ConditionTypeReady,
				v1beta1.ConditionReasonModuleCatalogIsReady)).To(BeTrue())
		}, Timeout, Interval)

		By("getting the label count of module templates in the remote cluster before updating the labels")
		Expect(kyma.Spec.Modules).NotTo(BeEmpty())
		count, err := GetModuleTemplatesLabelCount(runtimeClient, kyma, true)
		Expect(err).ShouldNot(HaveOccurred())

		By("updating a module template in the remote cluster to simulate unwanted modification")
		Eventually(ModifyModuleTemplateSpecThroughLabels(runtimeClient, kyma,
			ocmv1.Labels{ocmv1.Label{Name: "test", Value: json.RawMessage(`{"foo":"bar"}`)}},
			true), Timeout, Interval).Should(Succeed())

		By("verifying the discovered override and checking the reset label")
		Eventually(ModuleTemplatesLabelsCountMatch(
			runtimeClient, kyma, count, true), 20*time.Second, Interval).Should(Succeed())
	})
})
