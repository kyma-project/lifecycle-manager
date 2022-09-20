package v1alpha1_test

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

var testFiles = filepath.Join("..", "..", "config", "samples", "tests") //nolint:gochecknoglobals

var _ = Describe("Webhook ValidationCreate Strict", func() {
	SetupWebhook()
	data := unstructured.Unstructured{}
	data.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.OperatorPrefix,
		Version: v1alpha1.Version,
		Kind:    "SampleCRD",
	})
	It("should successfully fetch accept a moduletemplate based on a compliant crd", func() {
		crd := GetCRD(v1alpha1.OperatorPrefix, "samplecrd")
		Eventually(func() error {
			return k8sClient.Create(webhookServerContext, crd)
		}, "10s").Should(Succeed())

		template, err := test.ModuleTemplateFactory(v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, data)
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})

	It("should accept a moduletemplate based on a non-compliant crd in non-strict mode", func() {
		crd := GetNonCompliantCRD(v1alpha1.OperatorPrefix, "samplecrd")

		Eventually(func() error {
			return k8sClient.Create(webhookServerContext, crd)
		}, "10s").Should(Succeed())
		template, err := test.ModuleTemplateFactory(v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.ChannelStable,
		}, data)
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})
	StopWebhook()
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

func GetNonCompliantCRD(group, sample string) *v1.CustomResourceDefinition {
	crd := GetCRD(group, sample)
	crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"].Properties["state"] = v1.JSONSchemaProps{
		Type: "string",
		Enum: []v1.JSON{},
	}
	return crd
}
