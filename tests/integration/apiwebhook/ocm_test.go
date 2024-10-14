package apiwebhook_test

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"ocm.software/ocm/api/ocm/compdesc/versions/ocm.software/v3alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"Webhook Validation OCM Schemas", Ordered, func() {
		data := unstructured.Unstructured{}
		data.SetGroupVersionKind(
			schema.GroupVersionKind{
				Group:   shared.OperatorGroup,
				Version: v1beta2.GroupVersion.Version,
				Kind:    "SampleCRD",
			},
		)
		It(
			"should successfully fetch accept a moduletemplate based on template with a v3alpha1 ocm descriptor",
			func() {
				crd := GetCRD(shared.OperatorGroup, v1beta2.GroupVersion.Version, "samplecrd")
				Eventually(k8sClient.Create, Timeout, Interval).
					WithContext(webhookServerContext).
					WithArguments(crd).Should(Succeed())
				template := builder.NewModuleTemplateBuilder().
					WithLabelModuleName("test-module").
					WithChannel(v1beta2.DefaultChannel).
					WithModuleCR(&data).
					WithOCM(v3alpha1.SchemaVersion).Build()
				Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
				Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

				Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
			},
		)
	},
)
