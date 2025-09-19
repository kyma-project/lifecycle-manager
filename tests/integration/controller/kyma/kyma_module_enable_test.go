package kyma_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Given kyma CR with invalid module enabled", Ordered, func() {
	kyma := NewTestKyma("kyma")
	skrKyma := NewSKRKyma()
	var skrClient client.Client
	var err error
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
	})
	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})
	BeforeEach(func() {
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(skrClient, skrKyma).Should(Succeed())
	})
	It("KCP and remote Kymas eventually exist", func() {
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())
	})
	It("When enable module with channel and version, expect module status in Error state", func() {
		Skip("Version attribute is disabled for now on the CRD level")
		module := NewTestModuleWithChannelVersion("test", v1beta2.DefaultChannel, "1.0.0")
		Eventually(givenKymaWithModule, Timeout, Interval).
			WithArguments(kcpClient, kyma, skrClient, skrKyma, module).Should(Succeed())
		Eventually(expectKymaStatusModules(ctx, kyma, module.Name, shared.StateError), Timeout,
			Interval).Should(Succeed())
	})
	It("When enable module with none channel, expect module status become error", func() {
		module := NewTestModuleWithChannelVersion("test", string(shared.NoneChannel), "")
		Eventually(givenKymaWithModule, Timeout, Interval).
			WithArguments(kcpClient, kyma, skrClient, skrKyma, module).Should(Succeed())
		Eventually(expectKymaStatusModules(ctx, kyma, module.Name, shared.StateError), Timeout,
			Interval).Should(Succeed())
	})
})

func givenKymaWithModule(
	kcpClient client.Client,
	kcpKyma *v1beta2.Kyma,
	skrClient client.Client,
	remoteKyma *v1beta2.Kyma,
	module v1beta2.Module,
) error {
	if err := EnableModule(ctx, skrClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), module); err != nil {
		return err
	}
	Eventually(SyncKyma, Timeout, Interval).
		WithContext(ctx).WithArguments(kcpClient, kcpKyma).Should(Succeed())
	DeployModuleTemplates(ctx, kcpClient, kcpKyma)
	return nil
}
