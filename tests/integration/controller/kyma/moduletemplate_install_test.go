package kyma_test

import (
	"fmt"

	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("ModuleTemplate installation", func() {
	DescribeTable("Test Modules",
		func(givenCondition func(kyma *v1beta2.Kyma) error, expectedBehavior func(kyma *v1beta2.Kyma) error) {
			kyma := NewTestKyma("kyma")

			kyma.Spec.Modules = append(
				kyma.Spec.Modules, NewTestModule("test-module", v1beta2.DefaultChannel))
			Eventually(givenCondition, Timeout, Interval).WithArguments(kyma).Should(Succeed())
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
) func(kyma *v1beta2.Kyma) error {
	return func(kyma *v1beta2.Kyma) error {
		if kyma.Labels == nil {
			kyma.Labels = map[string]string{}
		}
		if isKymaInternal {
			kyma.Labels[v1beta2.InternalLabel] = v1beta2.EnableLabelValue
		}
		if isKymaBeta {
			kyma.Labels[v1beta2.BetaLabel] = v1beta2.EnableLabelValue
		}
		for _, module := range kyma.Spec.Modules {
			mtBuilder := builder.NewModuleTemplateBuilder().
				WithModuleName(module.Name).
				WithChannel(module.Channel).
				WithOCM(compdescv2.SchemaVersion)
			if isModuleTemplateInternal {
				mtBuilder.WithLabel(v1beta2.InternalLabel, v1beta2.EnableLabelValue)
			}
			if isModuleTemplateBeta {
				mtBuilder.WithLabel(v1beta2.BetaLabel, v1beta2.EnableLabelValue)
			}
			template := mtBuilder.Build()
			Eventually(controlPlaneClient.Create, Timeout, Interval).WithContext(ctx).
				WithArguments(template).
				Should(Succeed())
		}
		Eventually(controlPlaneClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma).Should(Succeed())
		return nil
	}
}

func expectManifestInstalled(shouldInstalled bool) func(kyma *v1beta2.Kyma) error {
	return func(kyma *v1beta2.Kyma) error {
		for _, module := range kyma.Spec.Modules {
			manifest, err := GetManifest(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
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
