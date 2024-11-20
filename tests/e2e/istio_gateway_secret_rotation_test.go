package e2e_test

import (
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Istio Gateway Secret Rotation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given KCP Cluster, rotated CA certificate, and Istio Gateway Secret", func() {
		It("Then Istio Gateway Secret is a copy of CA Certificate", func() {
			namespacedRootCASecretName := types.NamespacedName{
				Name:      "klm-watcher",
				Namespace: IstioNamespace,
			}

			// The timeout used is 4 minutes bec the certificate gets rotated every 1 minute
			Eventually(IstioGatewaySecretIsSyncedToRootCA, 4*time.Minute).
				WithContext(ctx).
				WithArguments(namespacedRootCASecretName, kcpClient).
				Should(Succeed())

			By("And LastModifiedAt timestamp is valid")
			gwSecret, err := gatewaysecret.GetGatewaySecret(ctx, kcpClient)
			Expect(err).NotTo(HaveOccurred())
			lastModifiedAtTime, err := gatewaysecret.GetValidLastModifiedAt(gwSecret)
			Expect(err).To(Succeed())

			By("And LastModifiedAt timestamp is updated")
			Eventually(GatewaySecretCreationTimeIsUpdated, 4*time.Minute).
				WithContext(ctx).
				WithArguments(lastModifiedAtTime, kcpClient).
				Should(Succeed())

			By("And new Istio Gateway Secret is also a copy of CA Certificate")
			Eventually(IstioGatewaySecretIsSyncedToRootCA, 4*time.Minute).
				WithContext(ctx).
				WithArguments(namespacedRootCASecretName, kcpClient).
				Should(Succeed())
		})
	})
})
