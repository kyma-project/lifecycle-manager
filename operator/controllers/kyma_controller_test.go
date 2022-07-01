package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Kyma Controller", func() {
	const (
		namespace = "default"
		timeout   = time.Second * 3
		interval  = time.Millisecond * 250
	)

	When("Creating a Kyma CR With No Modules", func() {
		kyma := &v1alpha1.Kyma{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       v1alpha1.KymaKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-module-kyma",
				Namespace: namespace,
			},
			Spec: v1alpha1.KymaSpec{
				Components: []v1alpha1.ComponentType{},
				Channel:    v1alpha1.DefaultChannel,
			},
		}

		kymaIsErrored := func() bool {
			createdKyma := &v1alpha1.Kyma{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
			if err != nil || createdKyma.Status.State != v1alpha1.KymaStateError {
				return false
			}

			return true
		}

		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, kyma)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, kyma)).Should(Succeed())
		})

		It("Should result in an error state", func() {
			By(fmt.Sprintf("having transitioned the CR State to %s as there are no modules", v1alpha1.KymaStateError))
			Eventually(kymaIsErrored, timeout, interval).Should(BeTrue())
		})
	})

	When("creating a Kyma CR with a set of modules", func() {
		kyma := &v1alpha1.Kyma{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       v1alpha1.KymaKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-module-kyma",
				Namespace: namespace,
			},
			Spec: v1alpha1.KymaSpec{
				Components: []v1alpha1.ComponentType{
					{
						Name:    "manifest",
						Channel: v1alpha1.ChannelStable,
					},
				},
				Channel: v1alpha1.DefaultChannel,
			},
		}
		kymaState := func() string {
			createdKyma := &v1alpha1.Kyma{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
			if err != nil {
				return ""
			}

			return string(createdKyma.Status.State)
		}

		activeModules := make([]*v1alpha1.ModuleTemplate, 0)

		BeforeEach(func() {
			for _, moduleSpec := range kyma.Spec.Components {
				moduleFileName := fmt.Sprintf("operator_v1alpha1_moduletemplate_%s_%s.yaml", moduleSpec.Name, moduleSpec.Channel)
				moduleFilePath := filepath.Join("..", "config", "samples", "component-integration-installed", moduleFileName)

				By(fmt.Sprintf("using %s for %s in %s", moduleFilePath, moduleSpec.Name, moduleSpec.Channel))

				moduleFile, err := os.ReadFile(moduleFilePath)
				Expect(err).To(BeNil())
				Expect(moduleFile).ToNot(BeEmpty())

				var moduleTemplate v1alpha1.ModuleTemplate
				Expect(yaml.Unmarshal(moduleFile, &moduleTemplate)).To(Succeed())

				By(fmt.Sprintf("creating the module %s", moduleFilePath))
				Expect(k8sClient.Create(ctx, moduleTemplate.DeepCopy())).To(Succeed())
				activeModules = append(activeModules, &moduleTemplate)
			}
		})
		BeforeEach(func() {
			Expect(k8sClient.Create(ctx, kyma)).Should(Succeed())
		})
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, kyma)).Should(Succeed())
		})

		It(fmt.Sprintf("should result in a %s state", v1alpha1.KymaStateReady), func() {
			By(fmt.Sprintf("checking the state subresource to be in %s", v1alpha1.KymaStateProcessing))
			Expect(k8sClient.Create(ctx, kyma))
			Eventually(kymaState, timeout, interval).Should(BeEquivalentTo(string(v1alpha1.KymaStateProcessing)))
			Consistently(kymaState, timeout, interval).Should(BeEquivalentTo(string(v1alpha1.KymaStateProcessing)))

			By("having created new conditions in its status")
			var updatedKyma v1alpha1.Kyma
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kyma), &updatedKyma)).To(Succeed())
			Expect(updatedKyma.Status.Conditions).To(HaveLen(len(activeModules) + 1))

			By("reacting to a change of its Modules")
			When(fmt.Sprintf("all of the modules change their state to %s", v1alpha1.KymaStateReady), func() {
				for _, activeModule := range activeModules {
					Eventually(func() error {
						activeUnstructuredModuleFromAPIServer := &unstructured.Unstructured{}
						activeUnstructuredModuleFromAPIServer.SetGroupVersionKind(activeModule.Spec.Data.GroupVersionKind())
						Expect(k8sClient.Get(ctx, client.ObjectKey{
							Namespace: namespace,
							Name:      activeModule.GetLabels()[labels.ControllerName] + kyma.Name,
						},
							activeUnstructuredModuleFromAPIServer),
						).To(Succeed())
						activeUnstructuredModuleFromAPIServer.Object[watch.Status] = map[string]interface{}{
							watch.State: v1alpha1.KymaStateReady,
						}

						return k8sManager.GetClient().Status().Update(ctx, activeUnstructuredModuleFromAPIServer)
					}, timeout, interval).Should(Succeed())
				}
			})

			By("having updated the Kyma CR state to ready")
			Eventually(kymaState, 10*time.Second, interval).Should(BeEquivalentTo(string(v1alpha1.KymaStateReady)))
		})
	})
})
