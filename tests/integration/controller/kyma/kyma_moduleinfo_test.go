package kyma_test

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrModuleNumberMismatch = errors.New("Spec.Modules number not match with Status.Modules")

var (
	kymaName   = "kyma"
	moduleName = "module"
)

var _ = Describe("Kyma module control", Ordered, func() {
	kyma := NewTestKyma(kymaName)
	skrKyma := NewSKRKyma()
	module := NewTestModule(moduleName, v1beta2.DefaultChannel)
	var skrClient client.Client
	var err error

	BeforeAll(func() {
		DeployModuleTemplates(ctx, kcpClient, &v1beta2.Kyma{Spec: v1beta2.KymaSpec{Modules: []v1beta2.Module{module}}})
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(func() error {
			skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
			return err
		}, Timeout, Interval).Should(Succeed())
		By("Waiting for KCP Kyma to exist")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())
		By("Waiting for SKR Kyma to exist")
		Eventually(KymaExists, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace()).
			Should(Succeed())
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), module).
			Should(Succeed())
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
		kymaName = kyma.GetName()
	})

	DescribeTable(
		"Test Manifests",
		func(givenCondition func(client.Client, string, string) error, 
		expectedBehavior func(client.Client, string, string) error) {
			Eventually(givenCondition, Timeout, Interval).
				WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace()).
				Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).
				WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace()).
				Should(Succeed())
		},
		Entry("When deploy module, expect number of Manifests matches spec.modules",
			noCondition(),
			expectCorrectNumberOfModuleStatus),
		Entry("When module state become ready, expect Manifests state become ready",
			updateManifestStateWrapper(),
			modulesHaveReadyStatus),
		Entry("When remove module in spec, expect number of Manifests matches spec.modules",
			removeModule(),
			expectCorrectNumberOfModuleStatus),
	)
})

func noCondition() func(client.Client, string, string) error {
	return func(_ client.Client, _, _ string) error {
		return nil
	}
}

func expectCorrectNumberOfModuleStatus(skrClient client.Client, kymaName string, kymaNamespace string) error {
	createdKyma, err := GetKyma(ctx, skrClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	if len(createdKyma.Spec.Modules) == len(createdKyma.Status.Modules) {
		return nil
	}
	return ErrModuleNumberMismatch
}

func updateManifestStateWrapper() func(client.Client, string, string) error {
	return func(skrClient client.Client, name, namespace string) error {
		skrKyma, err := GetKyma(ctx, skrClient, name, namespace)
		Expect(err).ShouldNot(HaveOccurred())
		for _, module := range skrKyma.Spec.Modules {
			err = UpdateManifestState(ctx, kcpClient, kymaName, ControlPlaneNamespace, module.Name, shared.StateReady)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func modulesHaveReadyStatus(skrClient client.Client, kymaName string, kymaNamespace string) error {
	Eventually(KymaIsInState, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kymaName, kymaNamespace, skrClient, shared.StateReady).
		Should(Succeed())
	skrKyma, err := GetKyma(ctx, skrClient, kymaName, kymaNamespace)
	Expect(err).ShouldNot(HaveOccurred())
	for _, module := range skrKyma.Spec.Modules {
		err = CheckModuleState(ctx, skrClient, kymaName, kymaNamespace, module.Name, shared.StateReady)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeModule() func(client.Client, string, string) error {
	return func(skrClient client.Client, kymaName, kymaNamespace string) error {
		createdKyma, err := GetKyma(ctx, skrClient, kymaName, kymaNamespace)
		Expect(err).ShouldNot(HaveOccurred())
		createdKyma.Spec.Modules = []v1beta2.Module{}
		return skrClient.Update(ctx, createdKyma)
	}
}
