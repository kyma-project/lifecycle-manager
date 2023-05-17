package v1beta2_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webhook Validation Kyma Create", Ordered, func() {
	It("should create kyma without duplicate module", func() {
		kyma := testutils.NewTestKyma("test-kyma")
		for _, module := range []string{"module1", "module2"} {
			kyma.Spec.Modules = append(
				kyma.Spec.Modules, v1beta2.Module{
					ControllerName: "manifest",
					Name:           module,
					Channel:        v1beta2.DefaultChannel,
				})
		}
		Expect(k8sClient.Create(webhookServerContext, kyma)).Should(Succeed())
	})
})
