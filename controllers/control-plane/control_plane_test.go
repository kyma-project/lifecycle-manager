package control_plane_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func registerControlPlaneLifecycleForKyma(kyma *v1beta1.Kyma) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(syncKyma(kyma), Timeout, Interval).Should(Succeed())
	})
}

func syncKyma(kyma *v1beta1.Kyma) error {
	err := controlPlaneClient.Get(ctx, client.ObjectKey{
		Name:      kyma.Name,
		Namespace: metav1.NamespaceDefault,
	}, kyma)
	// It might happen in some test case, kyma get deleted, if you need to make sure Kyma should exist,
	// write expected condition to check it specifically.
	return client.IgnoreNotFound(err)
}

var _ = Describe("Kyma with managed fields", Ordered, func() {
	kyma := NewTestKyma("managed-kyma")
	registerControlPlaneLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'lifecycle-manager'", func() {
		Eventually(ExpectKymaManagerField, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), v1beta1.OperatorName).
			Should(BeTrue())
	})
})
