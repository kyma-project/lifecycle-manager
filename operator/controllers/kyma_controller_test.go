package controllers_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	timeout   = time.Second * 3
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
		component := &unstructured.Unstructured{}
		component.SetGroupVersionKind(moduleTemplate.Spec.Data.GroupVersionKind())
		Expect(k8sClient.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      moduleTemplate.GetLabels()[v1alpha1.ModuleName] + kyma.GetName(),
		}, component),
		).To(Succeed())
		component.Object[watch.Status] = map[string]any{watch.State: string(state)}
		return k8sManager.GetClient().Status().Update(ctx, component)
	}
}

func GetModuleTemplate(sample string, module v1alpha1.Module, profile v1alpha1.Profile) *v1alpha1.ModuleTemplate {
	moduleFileName := fmt.Sprintf(
		"operator_v1alpha1_moduletemplate_%s_%s_%s_%s_%s.yaml",
		module.ControllerName,
		module.Name,
		module.Channel,
		string(profile),
		sample,
	)
	modulePath := filepath.Join("..", "config", "samples", "component-integration-installed", moduleFileName)
	By(fmt.Sprintf("using %s for %s in %s", modulePath, module.Name, module.Channel))

	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).To(BeNil())
	Expect(moduleFile).ToNot(BeEmpty())

	var moduleTemplate v1alpha1.ModuleTemplate
	Expect(yaml.Unmarshal(moduleFile, &moduleTemplate)).To(Succeed())
	return &moduleTemplate
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
			template := GetModuleTemplate("empty", module, v1alpha1.ProfileProduction)
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
