package v1beta2_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe(
	"Webhook Validation OCM Schemas", Ordered, func() {
		data := unstructured.Unstructured{}
		data.SetGroupVersionKind(
			schema.GroupVersionKind{
				Group:   v1beta2.OperatorPrefix,
				Version: v1beta2.GroupVersion.Version,
				Kind:    "SampleCRD",
			},
		)
		It(
			"should successfully fetch accept a moduletemplate based on template with a v3alpha1 ocm descriptor",
			func() {
				crd := GetCRD(v1beta2.OperatorPrefix, "samplecrd")
				Eventually(k8sClient.Create, Timeout, Interval).
					WithContext(webhookServerContext).
					WithArguments(crd).Should(Succeed())
				template, err := testutils.ModuleTemplateFactoryForSchema(
					v1beta2.Module{
						ControllerName: "manifest",
						Name:           "example-module-name",
						Channel:        v1beta2.DefaultChannel,
					}, data, v3alpha1.SchemaVersion, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
				Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

				Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
			},
		)
	},
)
