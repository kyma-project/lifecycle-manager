package controllers_test

import (
	"encoding/json"
	"time"

	"github.com/kyma-project/kyma-operator/operator/pkg/test"

	sampleCRDv1alpha1 "github.com/kyma-project/kyma-operator/operator/config/samples/component-integration-installed/crd/v1alpha1"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"
	manifestV1alpha1 "github.com/kyma-project/manifest-operator/operator/api/v1alpha1"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	timeout   = time.Second * 10
	interval  = time.Millisecond * 250
)

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       v1alpha1.KymaKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{},
			Channel: v1alpha1.DefaultChannel,
			Profile: v1alpha1.DefaultProfile,
		},
	}
}

func RegisterDefaultLifecycleForKyma(kyma *v1alpha1.Kyma) {
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, kyma)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, kyma)).Should(Succeed())
	})
}

func IsKymaInState(kyma *v1alpha1.Kyma, state v1alpha1.KymaState) func() bool {
	return func() bool {
		kymaFromCluster := &v1alpha1.Kyma{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      kyma.GetName(),
			Namespace: kyma.GetNamespace(),
		}, kymaFromCluster)
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}

		return true
	}
}

func GetKymaState(kyma *v1alpha1.Kyma) func() string {
	return func() string {
		createdKyma := &v1alpha1.Kyma{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kyma *v1alpha1.Kyma) func() []v1alpha1.KymaCondition {
	return func() []v1alpha1.KymaCondition {
		createdKyma := &v1alpha1.Kyma{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
		if err != nil {
			return []v1alpha1.KymaCondition{}
		}
		return createdKyma.Status.Conditions
	}
}

func UpdateModuleState(
	kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate, state v1alpha1.KymaState,
) func() error {
	return func() error {
		component, err := getModule(kyma, moduleTemplate)
		Expect(err).ShouldNot(HaveOccurred())
		component.Object[watch.Status] = map[string]any{watch.State: string(state)}
		return k8sManager.GetClient().Status().Update(ctx, component)
	}
}

func deleteModule(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate,
) error {
	component := moduleTemplate.Spec.Data.DeepCopy()
	if moduleTemplate.Spec.Target == v1alpha1.TargetRemote {
		component.SetKind("Manifest")
	}
	component.SetNamespace(namespace)
	component.SetName(parsed.CreateModuleName(moduleTemplate.GetLabels()[v1alpha1.ModuleName], kyma.GetName()))
	return k8sClient.Delete(ctx, component)
}

var _ = Describe("Kyma with no ModuleTemplate", func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in an error state", func() {
		By("having transitioned the CR State to Error as there are no modules")
		Eventually(IsKymaInState(kyma, v1alpha1.KymaStateError), timeout, interval).Should(BeTrue())
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
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("should result in Kyma becoming Ready", func() {
		By("checking the state to be Processing")
		Eventually(GetKymaState(kyma), 20*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.KymaStateProcessing)))

		By("having created new conditions in its status")
		Eventually(GetKymaConditions(kyma), timeout, interval).ShouldNot(BeEmpty())

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range moduleTemplates {
			Eventually(UpdateModuleState(kyma, activeModule, v1alpha1.KymaStateReady), timeout, interval).Should(Succeed())
		}

		By("having updated the Kyma CR state to ready")
		Eventually(GetKymaState(kyma), 20*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.KymaStateReady)))
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
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
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
			Expect(k8sClient.Create(ctx, template)).To(Succeed())
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
			err := k8sClient.Update(ctx, activeModule)
			Expect(err).ToNot(HaveOccurred())
		}
		By("CR updated with new value in spec.resource.spec")
		for _, activeModule := range moduleTemplates {
			Eventually(SKRModuleExistWithOverwrites(kyma, activeModule), timeout, interval).Should(Equal(valueUpdated))
		}
	})
})

func ModuleExist(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate) func() error {
	return func() error {
		_, err := getModule(kyma, moduleTemplate)
		return err
	}
}

func SKRModuleExistWithOverwrites(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate) func() string {
	return func() string {
		module, err := getModule(kyma, moduleTemplate)
		Expect(err).ToNot(HaveOccurred())
		body, err := json.Marshal(module.Object["spec"])
		Expect(err).ToNot(HaveOccurred())
		manifestSpec := manifestV1alpha1.ManifestSpec{}
		err = json.Unmarshal(body, &manifestSpec)
		Expect(err).ToNot(HaveOccurred())
		body, err = json.Marshal(manifestSpec.Resource.Object["spec"])
		Expect(err).ToNot(HaveOccurred())
		skrModuleSpec := sampleCRDv1alpha1.SKRModuleSpec{}
		err = json.Unmarshal(body, &skrModuleSpec)
		Expect(err).ToNot(HaveOccurred())
		return skrModuleSpec.InitKey
	}
}

func getModule(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate,
) (*unstructured.Unstructured, error) {
	component := moduleTemplate.Spec.Data.DeepCopy()
	if moduleTemplate.Spec.Target == v1alpha1.TargetRemote {
		component.SetKind("Manifest")
	}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      parsed.CreateModuleName(moduleTemplate.GetLabels()[v1alpha1.ModuleName], kyma.GetName()),
	}, component)
	if err != nil {
		return nil, err
	}
	return component, nil
}
