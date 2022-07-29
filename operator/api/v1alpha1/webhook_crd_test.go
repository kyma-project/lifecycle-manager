package v1alpha1_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

var testFiles = filepath.Join("..", "..", "config", "samples", "tests") //nolint:gochecknoglobals

var _ = Describe("Webhook ValidationCreate", func() {
	It("should successfully fetch accept a moduletemplate based on a compliant crd", func() {
		crd := GetCRD("component.kyma-project.io", "samplecrd")
		Eventually(func() error {
			return k8sClient.Create(ctx, crd)
		}, "10s").Should(Succeed())

		template := GetModuleTemplate("samplecrd", v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, v1alpha1.ProfileProduction)
		Expect(k8sClient.Create(ctx, template)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, template)).Should(Succeed())

		Expect(k8sClient.Delete(ctx, crd)).Should(Succeed())
	})

	It("should reject a moduletemplate based on a non-compliant crd", func() {
		crd := GetCRD("component.kyma-project.io", "samplecrd")

		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"].Properties["state"] = v1.JSONSchemaProps{
			Type: "string",
			Enum: []v1.JSON{},
		}

		Eventually(func() error {
			return k8sClient.Create(ctx, crd)
		}, "10s").Should(Succeed())

		template := GetModuleTemplate("samplecrd", v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, v1alpha1.ProfileProduction)
		Expect(k8sClient.Create(ctx, template)).Error().To(HaveField("ErrStatus.Message",
			ContainSubstring("is invalid: spec.data.status.state[enum]: Not found: \"Processing\"")))

		Expect(k8sClient.Delete(ctx, crd)).Should(Succeed())
	})
})

func GetModuleTemplate(sample string, module v1alpha1.Module, profile v1alpha1.Profile) *v1alpha1.ModuleTemplate {
	moduleFileName := fmt.Sprintf(
		"operator_v1alpha1_moduletemplate_%s_%s_%s_%s_%s.yaml",
		module.ControllerName,
		module.Name,
		module.Channel,
		string(profile),
		sample,
	)
	modulePath := filepath.Join(testFiles, "moduletemplates", moduleFileName)
	By(fmt.Sprintf("using %s for %s in %s", modulePath, module.Name, module.Channel))

	file, err := os.ReadFile(modulePath)
	Expect(err).To(BeNil())
	Expect(file).ToNot(BeEmpty())

	var moduleTemplate v1alpha1.ModuleTemplate
	Expect(yaml.Unmarshal(file, &moduleTemplate)).To(Succeed())
	return &moduleTemplate
}

func GetCRD(group, sample string) *v1.CustomResourceDefinition {
	crdFileName := fmt.Sprintf(
		"%s_%s.yaml",
		group,
		sample,
	)
	modulePath := filepath.Join(testFiles, "crds", crdFileName)
	By(fmt.Sprintf("using %s as CRD", modulePath))

	file, err := os.ReadFile(modulePath)
	Expect(err).To(BeNil())
	Expect(file).ToNot(BeEmpty())

	var crd v1.CustomResourceDefinition

	Expect(yaml.Unmarshal(file, &crd)).To(Succeed())
	return &crd
}
