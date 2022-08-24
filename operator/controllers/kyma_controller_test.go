package controllers_test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/kyma-operator/operator/pkg/test"

	"sigs.k8s.io/controller-runtime/pkg/client"

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
			template, err := test.ModuleTemplateFactory("empty", module, unstructured.Unstructured{})
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

		By("Kyma status contains expected condition")
		kymaInCluster, err := GetKyma(controlPlaneClient, kyma)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(kymaInCluster.ContainsCondition(v1alpha1.ConditionTypeReady,
			v1alpha1.ConditionReasonModulesAreReady, metav1.ConditionTrue)).To(BeTrue())
		By("Module Catalog created")
		Eventually(CatalogExists(controlPlaneClient, kyma), 10*time.Second, interval).Should(Succeed())
	})
})

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	var kyma *v1alpha1.Kyma
	moduleTemplates := make(map[string]*v1alpha1.ModuleTemplate)
	var skrModule *v1alpha1.Module
	var kcpModule *v1alpha1.Module
	BeforeAll(func() {
		kyma = NewTestKyma("kyma-test-recreate")
		skrModule = &v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "skr-module",
			Channel:        v1alpha1.ChannelStable,
		}
		kcpModule = &v1alpha1.Module{
			ControllerName: "kcp-operator",
			Name:           "kcp-module",
			Channel:        v1alpha1.ChannelStable,
		}
		kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule, *kcpModule)
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(ctx, client.ObjectKey{Name: kyma.Name, Namespace: namespace}, kyma)).Should(Succeed())
	})

	It("module template created", func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory("recreate", module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates[module.Name] = template
		}
	})

	It("CR should be recreated after delete", func() {
		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(kyma, activeModule), timeout, interval).Should(BeTrue())
		}
		By("Delete CR")
		for _, activeModule := range moduleTemplates {
			Expect(deleteModule(kyma, activeModule)).To(Succeed())
		}

		By("CR created again")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(kyma, activeModule), timeout, interval).Should(BeTrue())
		}
	})

	It("CR should be deleted after removed from kyma.spec.modules", func() {
		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(kyma, activeModule), timeout, interval).Should(BeTrue())
		}
		By("Remove kcp-module from kyma.spec.modules")
		kyma.Spec.Modules = []v1alpha1.Module{
			*skrModule,
		}
		Expect(controlPlaneClient.Update(ctx, kyma.SetObservedGeneration())).To(Succeed())

		By("kcp-module deleted")
		Eventually(ModuleNotExist(kyma, moduleTemplates[kcpModule.Name]), timeout, interval).Should(BeTrue())

		By("skr-module exists")
		Eventually(ModuleExists(kyma, moduleTemplates[skrModule.Name]), timeout, interval).Should(BeTrue())
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
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
			template, err := test.ModuleTemplateFactory("update", module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(kyma, activeModule), timeout, interval).Should(BeTrue())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range moduleTemplates {
			Eventually(UpdateModuleState(kyma, activeModule, v1alpha1.StateReady), 20*time.Second, interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState(kyma), 20*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.StateReady)))

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
			template, err := test.ModuleTemplateFactory("remote-kyma", module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma), 30*time.Second, interval).Should(Succeed())

		By("CR created")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(kyma, activeModule), timeout, interval).Should(BeTrue())
		}

		By("Remote Module Catalog created")
		Eventually(CatalogExists(runtimeClient, kyma), 30*time.Second, interval).Should(Succeed())
	})
})
