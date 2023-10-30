package v1beta2_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var testFiles = filepath.Join("..", "..", "config", "samples", "tests") //nolint:gochecknoglobals

var _ = Describe("Webhook ValidationCreate Strict", Ordered, func() {
	sampleCR := builder.NewSampleCRBuilder().Build()
	It("should successfully fetch accept a moduletemplate based on a compliant crd", func() {
		crd := GetCRD(v1beta2.OperatorPrefix, v1beta2.GroupVersion.Version, "samplecrd")
		Eventually(k8sClient.Create, Timeout, Interval).
			WithContext(webhookServerContext).
			WithArguments(crd).Should(Succeed())

		template := builder.NewModuleTemplateBuilder().
			WithModuleName("test-module").
			WithModuleCR(sampleCR).
			WithChannel(v1beta2.DefaultChannel).
			WithOCM(compdesc2.SchemaVersion).Build()
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})

	It("should accept a moduletemplate based on a non-compliant crd in non-strict mode", func() {
		crd := GetNonCompliantCRD(v1beta2.OperatorPrefix, v1beta2.GroupVersion.Version, "samplecrd")

		Eventually(k8sClient.Create, Timeout, Interval).
			WithContext(webhookServerContext).
			WithArguments(crd).Should(Succeed())
		template := builder.NewModuleTemplateBuilder().
			WithModuleName("test-module").
			WithModuleCR(sampleCR).
			WithChannel(v1beta2.DefaultChannel).
			WithOCM(compdesc2.SchemaVersion).Build()
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
		Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

		Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
	})

	It("should deny a version downgrade when updating", func() {
		crd := GetCRD(v1beta2.OperatorPrefix, v1beta2.GroupVersion.Version, "samplecrd")
		Eventually(k8sClient.Create, Timeout, Interval).
			WithContext(webhookServerContext).
			WithArguments(crd).Should(Succeed())
		template := builder.NewModuleTemplateBuilder().
			WithModuleName("test-module").
			WithModuleCR(sampleCR).
			WithChannel(v1beta2.DefaultChannel).
			WithOCM(compdesc2.SchemaVersion).Build()
		Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())

		descriptor, err := template.GetDescriptor()
		Expect(err).ToNot(HaveOccurred())
		version, err := semver.NewVersion(descriptor.Version)
		Expect(err).ToNot(HaveOccurred())
		descriptor.Version = fmt.Sprintf("v%d.%d.%d", version.Major(), version.Minor(), version.Patch()-1)
		for i := range descriptor.Resources {
			descriptor.Resources[i].SetVersion(descriptor.Version)
		}
		newDescriptor, err := compdesc.Encode(descriptor.ComponentDescriptor, compdesc.DefaultJSONLCodec)
		Expect(err).ToNot(HaveOccurred())
		template.Spec.Descriptor.Raw = newDescriptor

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

func GetCRD(group, version, sample string) *v1.CustomResourceDefinition {
	crdFileName := fmt.Sprintf(
		"%s_%s_%s.yaml",
		group,
		version,
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

func GetNonCompliantCRD(group, version, sample string) *v1.CustomResourceDefinition {
	crd := GetCRD(group, version, sample)
	crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"].Properties["state"] = v1.JSONSchemaProps{
		Type: "string",
		Enum: []v1.JSON{},
	}
	return crd
}
