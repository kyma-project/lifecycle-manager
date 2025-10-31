package kyma_test

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("ModuleTemplate installation", Ordered, func() {
	kyma := NewTestKyma("something")
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
	DescribeTable("Test Modules",
		func(givenCondition func(client.Client, *v1beta2.Kyma) error, expectedBehavior func(*v1beta2.Kyma) error) {
			skrKyma.Spec.Modules = append(
				skrKyma.Spec.Modules, NewTestModule("test-module", v1beta2.DefaultChannel))
			Eventually(givenCondition, Timeout, Interval).WithArguments(skrClient, skrKyma).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).WithArguments(kyma).Should(Succeed())
		},
		Entry("With non-internal kyma and internal ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(false, false, true, false),
			expectManifestInstalled(false)),
		Entry("With internal kyma and internal ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, false, true, false),
			expectManifestInstalled(true)),
		Entry("With non-beta kyma and beta ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(false, false, false, true),
			expectManifestInstalled(false)),
		Entry("With beta kyma and beta ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(false, true, false, true),
			expectManifestInstalled(false)),
		Entry("With beta kyma and internal ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(false, true, true, false),
			expectManifestInstalled(false)),
		Entry("With default kyma and default ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(false, false, false, false),
			expectManifestInstalled(true)),
		Entry("With beta kyma and default ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(false, true, false, false),
			expectManifestInstalled(true)),
		Entry("With internal kyma and default ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, false, false, false),
			expectManifestInstalled(true)),
		Entry("With internal beta kyma and default ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, true, false, false),
			expectManifestInstalled(true)),
		Entry("With beta kyma and beta ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(false, true, false, true),
			expectManifestInstalled(true)),
		Entry("With internal kyma and beta ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(true, false, false, true),
			expectManifestInstalled(false)),
		Entry("With default kyma and beta internal ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(false, false, true, true),
			expectManifestInstalled(false)),
		Entry("With internal beta kyma and beta ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, true, false, true),
			expectManifestInstalled(true)),
		Entry("With internal beta kyma and internal ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, true, true, false),
			expectManifestInstalled(true)),
		Entry("With internal kyma and internal beta ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(true, false, true, true),
			expectManifestInstalled(false)),
		Entry("With beta kyma and internal beta ModuleTemplate, expect no manifest installed",
			givenKymaAndModuleTemplateCondition(false, true, true, true),
			expectManifestInstalled(false)),
		Entry("With internal beta kyma and internal beta ModuleTemplate, expect manifest installed",
			givenKymaAndModuleTemplateCondition(true, true, true, true),
			expectManifestInstalled(true)),
	)
})

func givenKymaAndModuleTemplateCondition(
	isKymaInternal,
	isKymaBeta,
	isModuleTemplateInternal,
	isModuleTemplateBeta bool,
) func(client.Client, *v1beta2.Kyma) error {
	return func(skrClient client.Client, skrKyma *v1beta2.Kyma) error {
		for _, module := range skrKyma.Spec.Modules {
			mtBuilder := builder.NewModuleTemplateBuilder().
				WithNamespace(ControlPlaneNamespace).
				WithModuleName(module.Name).
				WithChannel(module.Channel)
			if isModuleTemplateInternal {
				mtBuilder.WithLabel(shared.InternalLabel, shared.EnableLabelValue)
			}
			if isModuleTemplateBeta {
				mtBuilder.WithLabel(shared.BetaLabel, shared.EnableLabelValue)
			}
			template := mtBuilder.Build()
			Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
				WithArguments(template).
				Should(Succeed())
		}

		// wrap all the modifications to the skrKyma for later use
		kymaUpdateFunc := func(kyma *v1beta2.Kyma) error {
			if skrKyma.Labels == nil {
				skrKyma.Labels = map[string]string{}
			}
			if isKymaInternal {
				skrKyma.Labels[shared.InternalLabel] = shared.EnableLabelValue
			}
			if isKymaBeta {
				skrKyma.Labels[shared.BetaLabel] = shared.EnableLabelValue
			}
			return nil
		}

		Eventually(UpdateKymaWithFunc, Timeout, Interval).
			WithContext(ctx).
			WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), kymaUpdateFunc).
			Should(Succeed())

		return nil
	}
}

func expectManifestInstalled(shouldInstalled bool) func(*v1beta2.Kyma) error {
	return func(kyma *v1beta2.Kyma) error {
		for _, module := range kyma.Spec.Modules {
			manifest, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
			if shouldInstalled && manifest != nil {
				return nil
			}
			if !shouldInstalled && manifest == nil {
				return nil
			}
			if err != nil {
				return fmt.Errorf("found unexpected manifest %w", err)
			}
		}
		return nil
	}
}
