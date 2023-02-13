package v1alpha1_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

var testFiles = filepath.Join("..", "..", "config", "samples", "tests") //nolint:gochecknoglobals

var _ = Describe("Webhook ValidationCreate Strict", Ordered, func() {
	data := unstructured.Unstructured{}
	data.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1beta1.OperatorPrefix,
		Version: v1beta1.GroupVersion.Version,
		Kind:    "SampleCRD",
	})
	It("should successfully fetch accept a moduletemplate based on a compliant crd", func() {
		crd := GetCRD(v1beta1.OperatorPrefix, "samplecrd")
		Eventually(func() error {
			return k8sClient.Create(webhookServerContext, crd)
		}, Timeout, Interval).Should(Succeed())

		template, err := testutils.ModuleTemplateFactory(
			v1beta1.Module{
				ControllerName: "manifest",
				Name:           "example-module-name",
				Channel:        v1beta1.DefaultChannel,
			}, data,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})

	It("should accept a moduletemplate based on a non-compliant crd in non-strict mode", func() {
		crd := GetNonCompliantCRD(v1beta1.OperatorPrefix, "samplecrd")

		Eventually(func() error {
			return k8sClient.Create(webhookServerContext, crd)
		}, Timeout, Interval).Should(Succeed())
		template, err := testutils.ModuleTemplateFactory(
			v1beta1.Module{
				ControllerName: "manifest",
				Name:           "example-module-name",
				Channel:        v1beta1.DefaultChannel,
			}, data,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})

	It("should deny a version downgrade when updating", func() {
		crd := GetCRD(v1beta1.OperatorPrefix, "samplecrd")
		Eventually(func() error {
			return k8sClient.Create(webhookServerContext, crd)
		}, Timeout, Interval).Should(Succeed())

		template, err := testutils.ModuleTemplateFactory(
			v1beta1.Module{
				ControllerName: "manifest",
				Name:           "example-module-name",
				Channel:        v1beta1.DefaultChannel,
			}, data,
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())

		Expect(template.Spec.ModifyDescriptor(v1beta1.ModifyDescriptorVersion(func(version *semver.Version) string {
			return fmt.Sprintf("%v.%v.%v", version.Major(), version.Minor(), version.Patch()-1)
		}))).ToNot(HaveOccurred())

		err = k8sClient.Update(webhookServerContext, template)

		Expect(err).To(HaveOccurred())
		var statusErr *k8serrors.StatusError
		isStatusErr := errors.As(err, &statusErr)
		Expect(isStatusErr).To(BeTrue())
		Expect(statusErr.ErrStatus.Status).To(Equal("Failure"))
		Expect(string(statusErr.ErrStatus.Reason)).To(Equal("Invalid"))
		Expect(statusErr.ErrStatus.Message).
			To(ContainSubstring("version of templates can never be decremented "))

		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	},
	)
},
)

func GetCRD(group, sample string) *v1.CustomResourceDefinition {
	crdFileName := fmt.Sprintf(
		"%s_%s.yaml",
		group,
		sample,
	)
	modulePath := filepath.Join(testFiles, "crds", crdFileName)
	By(fmt.Sprintf("using %s as CRD", modulePath))

	file, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
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
