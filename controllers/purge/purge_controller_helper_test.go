package purge_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta1.Kyma) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})
}
