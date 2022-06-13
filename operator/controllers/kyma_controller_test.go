package controllers

import (
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("Kyma Controller", func() {
	const (
		namespace = "default"
		timeout   = time.Second * 10
		interval  = time.Millisecond * 250
	)

	Context("When Creating a Kyma CR With No Modules", func() {
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

		It("Should first create the Resource", func() {
			By("Creating the CR")
			Expect(k8sClient.Create(ctx, kyma)).Should(Succeed())

			kymaIsCreated := func() bool {
				createdKyma := &v1alpha1.Kyma{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
				if err != nil {
					return false
				}
				return true
			}

			Eventually(kymaIsCreated, timeout, interval).Should(BeTrue())
		})

		It(fmt.Sprintf("Should transition the CR State to %s as there are no modules", v1alpha1.KymaStateError), func() {
			kymaIsErrored := func() bool {
				createdKyma := &v1alpha1.Kyma{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
				if err != nil || createdKyma.Status.State != v1alpha1.KymaStateError {
					return false
				}
				return true
			}
			Eventually(kymaIsErrored, timeout, interval).Should(BeTrue())

			By("Checking that CR stays in Error")
			kymaState := func() string {
				createdKyma := &v1alpha1.Kyma{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, createdKyma)
				if err != nil {
					return ""
				}
				return string(createdKyma.Status.State)
			}
			Consistently(kymaState, timeout, interval).Should(Equal(string(v1alpha1.KymaStateError)))
		})
	})
})
