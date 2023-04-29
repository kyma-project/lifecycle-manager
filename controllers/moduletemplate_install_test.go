package controllers_test

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test ModuleTemplate installation", func() {
	DescribeTable("Test Modules",
		func(givenCondition func(kyma *v1beta2.Kyma) error, expectedBehavior func(kyma *v1beta2.Kyma) error) {
			kyma := NewTestKyma("kyma")

			kyma.Spec.Modules = append(
				kyma.Spec.Modules, v1beta2.Module{
					ControllerName: "manifest",
					Name:           NewUniqModuleName(),
					Channel:        v1beta2.DefaultChannel,
				})
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
			kyma.Labels[v1beta2.InternalLabel] = v1beta2.ActiveLabelValue
		}
		if isKymaBeta {
			kyma.Labels[v1beta2.BetaLabel] = v1beta2.ActiveLabelValue
		}
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, false, isModuleTemplateInternal, isModuleTemplateBeta)
		Eventually(controlPlaneClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma).Should(Succeed())
		return nil
	}
}

func expectManifestInstalled(shouldInstalled bool) func(kyma *v1beta2.Kyma) error {
	return func(kyma *v1beta2.Kyma) error {
		for _, module := range kyma.Spec.Modules {
			manifest, err := GetManifest(kyma, module)
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
