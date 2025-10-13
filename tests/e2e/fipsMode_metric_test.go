package e2e_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
)

var _ = Describe("FIPS Mode metric", Ordered, func() {
	Context("Given KCP Cluster", func() {
		It("When KLM is initialized", func() {
			By("Then fipsMode metrics is set to \"FipsModeOnly\"")
			Eventually(GetFipsModeGauge).
				WithContext(ctx).
				Should(Equal(metrics.FipsModeOnly))
		})
	})

	Context("Given SKR Cluster", func() {
		It("When Runtime Watcher is initialized", func() {
			By("Then fipsMode env exists in the webhook deployment")
			Eventually(func() error {
				skrWebhook, err := GetDeployment(ctx, skrClient, skrwebhookresources.SkrResourceName, RemoteNamespace)
				if err != nil {
					return err
				}
				for _, container := range skrWebhook.Spec.Template.Spec.Containers {
					if container.Name == "server" {
						for _, env := range container.Env {
							if env.Name == "GODEBUG" && env.Value == "fips140=only,tlsmlkem=0" {
								return nil
							}
						}
					}
				}
				return fmt.Errorf("GODEBUG env not found in the webhook deployment")
			}).Should(Succeed())
		})
	})
})
