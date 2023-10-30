package v1beta2_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
)

var _ = Describe(
	"Webhook Validation OCM Schemas", Ordered, func() {
		It(
			"should successfully fetch accept a moduletemplate based on template with a v3alpha1 ocm descriptor",
			func() {
				crd := GetCRD(v1beta2.OperatorPrefix, v1beta2.GroupVersion.Version, "samplecrd")
				Eventually(k8sClient.Create, Timeout, Interval).
					WithContext(webhookServerContext).
					WithArguments(crd).Should(Succeed())
				template := builder.NewModuleTemplateBuilder().
					WithModuleName("test-module").
					WithChannel(v1beta2.DefaultChannel).
					WithModuleCR(builder.NewSampleCRBuilder().Build()).
					WithOCM(v3alpha1.SchemaVersion).Build()
				Expect(k8sClient.Create(webhookServerContext, template)).Should(Succeed())
				Expect(k8sClient.Delete(webhookServerContext, template)).Should(Succeed())

				Expect(k8sClient.Delete(webhookServerContext, crd)).Should(Succeed())
			},
		)
	},
)
