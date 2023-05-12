package v1beta2_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Webhook Validation Kyma Create", Ordered, func() {
	It("should not create kyma contains duplicate module", func() {
		kyma := testutils.NewTestKyma("test-kyma")
		for _, module := range []string{"module1", "module1"} {
			kyma.Spec.Modules = append(
				kyma.Spec.Modules, v1beta2.Module{
					ControllerName: "manifest",
					Name:           module,
					Channel:        v1beta2.DefaultChannel,
				})
		}
		err := k8sClient.Create(webhookServerContext, kyma)

		Expect(err).To(HaveOccurred())
		var statusErr *k8serrors.StatusError
		isStatusErr := errors.As(err, &statusErr)
		Expect(isStatusErr).To(BeTrue())
		Expect(statusErr.ErrStatus.Status).To(Equal("Failure"))
		Expect(string(statusErr.ErrStatus.Reason)).To(Equal("Invalid"))
		Expect(statusErr.ErrStatus.Message).
			To(ContainSubstring(v1beta2.ErrDuplicateModule.Error()))
	})

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
