package controllers_test

import (
	"time"

	"github.com/kyma-project/kyma-operator/operator/pkg/test"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
)

const (
	namespace = "default"
	timeout   = time.Second * 10
	interval  = time.Millisecond * 250
)

func deleteModule(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate,
) error {
	component := moduleTemplate.Spec.Data.DeepCopy()
	if moduleTemplate.Spec.Target == v1alpha1.TargetRemote {
		component.SetKind("Manifest")
	}
	component.SetNamespace(namespace)
	component.SetName(parsed.CreateModuleName(moduleTemplate.GetLabels()[v1alpha1.ModuleName], kyma.GetName()))
	return controlPlaneClient.Delete(ctx, component)
}

var _ = Describe("Kyma with no ModuleTemplate", func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in an error state", func() {
		By("having transitioned the CR State to Error as there are no modules")
		Eventually(IsKymaInState(kyma, v1alpha1.StateError), timeout, interval).Should(BeTrue())
	})

	When("creating a Kyma CR with Module based on an Empty ModuleTemplate", func() {
	})
})

var _ = Describe("Kyma with empty ModuleTemplate", func() {
	kyma := NewTestKyma("empty-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "example-module-name",
		Channel:        v1alpha1.ChannelStable,
	})

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeEach(func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory("empty", module,
				v1alpha1.ProfileProduction, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("should result in Kyma becoming Ready", func() {
		By("checking the state to be Processing")
		Eventually(GetKymaState(kyma), 20*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.StateProcessing)))

		By("having created new conditions in its status")
		Eventually(GetKymaConditions(kyma), timeout, interval).ShouldNot(BeEmpty())

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range moduleTemplates {
			Eventually(UpdateModuleState(kyma, activeModule, v1alpha1.StateReady), 20*time.Second, interval).Should(Succeed())
		}

		By("having updated the Kyma CR state to ready")
		Eventually(GetKymaState(kyma), 20*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.StateReady)))
	})
})

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	kyma := NewTestKyma("kyma-test-recreate")
	RegisterDefaultLifecycleForKyma(kyma)

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-module",
		Channel:        v1alpha1.ChannelStable,
	}, v1alpha1.Module{
		ControllerName: "kcp-operator",
		Name:           "cp-module",
		Channel:        v1alpha1.ChannelStable,
	})

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeAll(func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory("recreate", module,
				v1alpha1.ProfileProduction, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("CR should be recreated after delete", func() {
		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExist(kyma, activeModule), timeout, interval).Should(Succeed())
		}
		By("Delete CR")
		for _, activeModule := range moduleTemplates {
			Expect(deleteModule(kyma, activeModule)).To(Succeed())
		}

		By("CR created again")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExist(kyma, activeModule), timeout, interval).Should(Succeed())
		}
	})
})

var _ = Describe("Kyma update Manifest CR", func() {
	kyma := NewTestKyma("kyma-test-update")
	RegisterDefaultLifecycleForKyma(kyma)

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-update",
		Channel:        v1alpha1.ChannelStable,
	})

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeEach(func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory("update", module,
				v1alpha1.ProfileProduction, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExist(kyma, activeModule), timeout, interval).Should(Succeed())
		}

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		for _, activeModule := range moduleTemplates {
			activeModule.Spec.Data.Object["spec"] = map[string]any{"initKey": valueUpdated}
			err := controlPlaneClient.Update(ctx, activeModule)
			Expect(err).ToNot(HaveOccurred())
		}
		By("CR updated with new value in spec.resource.spec")
		for _, activeModule := range moduleTemplates {
			Eventually(SKRModuleExistWithOverwrites(kyma, activeModule), timeout, interval).Should(Equal(valueUpdated))
		}
	})
})

var _ = Describe("Kyma sync into Remote Cluster", func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}

	RegisterDefaultLifecycleForKyma(kyma)

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-remote-module",
		Channel:        v1alpha1.ChannelStable,
	})

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeEach(func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory("remote-kyma", module,
				v1alpha1.ProfileProduction, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma), "30s", interval).Should(Succeed())

		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExist(kyma, activeModule), timeout, interval).Should(Succeed())
		}

		By("Remote Module Catalog created")
		Eventually(RemoteCatalogExists(runtimeClient, kyma), "30s", interval).Should(Succeed())
	})
})
