package e2e_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Self Signed Certificate Rotation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given Kyma deployed in KCP", func() {
		It("When self signed certificate exists", func() {
			certName := types.NamespacedName{
				Name:      watcher.ResolveTLSCertName(kyma.Name),
				Namespace: "istio-system",
			}
			Eventually(func() error {
				_, err := GetCACertificate(ctx, certName, kcpClient)
				return err
			}).Should(Succeed())
		})
		It("Then disable cert manager operator to prevent certificate auto renewed", func() {
			Eventually(StopDeployment).
				WithContext(ctx).
				WithArguments(kcpClient, "cert-manager", "cert-manager").
				Should(Succeed())
		})
		It(fmt.Sprintf("Then %s metric increased to 1", metrics.SelfSignedCertNotRenewMetrics), func() {
			Eventually(GetSelfSignedCertNotRenewMetricsGauge).
				WithContext(ctx).
				WithTimeout(5 * time.Minute).
				WithArguments(kyma.GetName()).
				Should(Equal(1))
		})
		It("Then enable cert manager operator to renew certificate", func() {
			Eventually(EnableDeployment).
				WithContext(ctx).
				WithArguments(kcpClient, "cert-manager", "cert-manager").
				Should(Succeed())
		})
		It(fmt.Sprintf("Then %s metric deleted", metrics.SelfSignedCertNotRenewMetrics), func() {
			Eventually(GetSelfSignedCertNotRenewMetricsGauge).
				WithContext(ctx).
				WithTimeout(5 * time.Minute).
				WithArguments(kyma.GetName()).
				Should(Equal(0))
		})
	})
})
