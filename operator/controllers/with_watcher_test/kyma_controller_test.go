package controllers_with_watcher_test

import (
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers/test_helper"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	//timeout   = time.Second * 10
	//interval  = time.Millisecond * 250
)

var _ = Describe("Kyma with no ModuleTemplate", Ordered, func() {
	kyma := test_helper.NewTestKyma("no-module-kyma")
	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	BeforeEach(func() {
		Expect(controlPlaneClient.Create(ctx, watcherCrForKyma)).To(Succeed())
	})
	AfterEach(func() {
		Expect(controlPlaneClient.Delete(ctx, watcherCrForKyma)).To(Succeed())
	})
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in a ready state immediately", func() {
		By("having transitioned the CR State to Ready as there are no modules")
		Eventually(IsKymaInState(kyma.GetName(), v1alpha1.StateReady), timeout, interval).Should(BeTrue())
	})
})

func DeployModuleTemplates(kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
	}
}

func DeleteModuleTemplates(kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Delete(ctx, template)).To(Succeed())
	}
}

func RegisterDefaultLifecycleForKyma(kyma *v1alpha1.Kyma) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
		DeployModuleTemplates(kyma)
	})

	AfterAll(func() {
		DeleteModuleTemplates(kyma)
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(ctx, client.ObjectKey{Name: kyma.Name, Namespace: namespace}, kyma)).Should(Succeed())
	})
}

func GetKyma(
	testClient client.Client,
	kymaName string,
) (*v1alpha1.Kyma, error) {
	kymaInCluster := &v1alpha1.Kyma{}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      kymaName,
	}, kymaInCluster)
	if err != nil {
		return nil, err
	}
	return kymaInCluster, nil
}

func GetKymaState(kymaName string) func() string {
	return func() string {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kymaName string) func() []metav1.Condition {
	return func() []metav1.Condition {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return []metav1.Condition{}
		}
		return createdKyma.Status.Conditions
	}
}

func IsKymaInState(kymaName string, state v1alpha1.State) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}
		return true
	}
}
