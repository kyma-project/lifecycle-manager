package v1alpha1_test

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/kyma-operator/operator/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
)

var testFiles = filepath.Join("..", "..", "config", "samples", "tests") //nolint:gochecknoglobals

var _ = Describe("Webhook ValidationCreate", func() {
	data := unstructured.Unstructured{}
	data.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.ComponentPrefix,
		Version: v1alpha1.Version,
		Kind:    "SampleCRD",
	})
	It("should successfully fetch accept a moduletemplate based on a compliant crd", func() {
		crd := GetCRD("component.kyma-project.io", "samplecrd")
		Eventually(func() error {
			return k8sClient.Create(ctx, crd)
		}, "10s").Should(Succeed())

		template, err := test.ModuleTemplateFactory("samplecrd", v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, data)
		Expect(err).ToNot(HaveOccurred())
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
		template, err := test.ModuleTemplateFactory("samplecrd-non-complaint", v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, data)
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(ctx, template)).Error().To(HaveField("ErrStatus.Message",
			ContainSubstring("is invalid: spec.data.status.state[enum]: Not found: \"Processing\"")))

		Expect(k8sClient.Delete(ctx, crd)).Should(Succeed())
	})
})

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
