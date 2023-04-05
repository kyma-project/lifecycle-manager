package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe(
	"Webhook Validation OCM Schemas", Ordered, func() {
		data := unstructured.Unstructured{}
		data.SetGroupVersionKind(
			schema.GroupVersionKind{
				Group:   v1beta1.OperatorPrefix,
				Version: v1beta1.GroupVersion.Version,
				Kind:    "SampleCRD",
			},
		)
		It(
			"should successfully fetch accept a moduletemplate based on template with a v3alpha1 ocm descriptor",
			func() {
				crd := GetCRD(v1beta1.OperatorPrefix, "samplecrd")
				Eventually(
					func() error {
						return k8sClient.Create(webhookServerContext, crd)
					}, Timeout, Interval,
				).Should(Succeed())

				template, err := testutils.ModuleTemplateFactoryForSchema(
					v1beta1.Module{
						ControllerName: "manifest",
						Name:           "example-module-name",
						Channel:        v1beta1.DefaultChannel,
					}, data, v3alpha1.SchemaVersion,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
				Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

				Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
			},
		)
	},
)
