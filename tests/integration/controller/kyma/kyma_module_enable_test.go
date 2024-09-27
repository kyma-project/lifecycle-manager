package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Given kyma CR with invalid module enabled", Ordered, func() {
	kyma := NewTestKyma("kyma")
	BeforeAll(func() {
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})

	It("When enable module with channel and version, expect module status in Error state", func() {
		Skip("Version attribute is disabled for now on the CRD level")
		module := NewTestModuleWithChannelVersion("test", v1beta2.DefaultChannel, "1.0.0")
		Eventually(givenKymaWithModule, Timeout, Interval).
			WithArguments(kyma, module).Should(Succeed())
		Eventually(expectKymaStatusModules(ctx, kyma, module.Name, shared.StateError), Timeout,
			Interval).Should(Succeed())
	})
	It("When enable module with none channel, expect module status become error", func() {
		module := NewTestModuleWithChannelVersion("test", string(shared.NoneChannel), "")
		Eventually(givenKymaWithModule, Timeout, Interval).
			WithArguments(kyma, module).Should(Succeed())
		Eventually(expectKymaStatusModules(ctx, kyma, module.Name, shared.StateError), Timeout,
			Interval).Should(Succeed())
	})
})

func givenKymaWithModule(kyma *v1beta2.Kyma, module v1beta2.Module) error {
	if err := EnableModule(ctx, kcpClient, kyma.Name, kyma.Namespace, module); err != nil {
		return err
	}
	Eventually(SyncKyma, Timeout, Interval).
		WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	DeployModuleTemplates(ctx, kcpClient, kyma)
	return nil
}
