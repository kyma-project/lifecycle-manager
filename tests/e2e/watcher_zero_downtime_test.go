package e2e_test

import (
	"context"
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Watcher Zero Downtime", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module Resources are deployed on SKR cluster", func() {
			By("And Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("And Module Operator Deployment is ready")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When SKR metrics service is exposed", func() {
			Expect(PatchServiceToTypeLoadBalancer(os.Getenv(skrConfigEnvVar),
				"skr-webhook-metrics", "kyma-system")).
				To(Succeed())
		})

		It("Then no downtime errors can be observed", func() {
			// Eventually because exposed metrics are not immediately available
			Eventually(triggerWatcherAndCheckDowntime).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace).
				Should(Succeed())
			Consistently(triggerWatcherAndCheckDowntime).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace).
				WithTimeout(4 * time.Minute).
				Should(Succeed())
		})
	})
})

func triggerWatcherAndCheckDowntime(ctx context.Context, skrClient client.Client, kymaName, kymaNamespace string) error {
	// Triggering watcher request
	kyma, err := GetKyma(ctx, skrClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	if kyma.Spec.Channel == v1beta2.DefaultChannel {
		err = UpdateKymaModuleChannel(ctx, skrClient, kymaName, kymaNamespace, FastChannel)
	} else {
		err = UpdateKymaModuleChannel(ctx, skrClient, kymaName, kymaNamespace, v1beta2.DefaultChannel)
	}
	if err != nil {
		return err
	}

	// Checking if failed KCP error metrics is not increasing
	count, err := GetWatcherFailedKcpTotalMetric(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("watcher is experiencing downtime")
	}
	return nil
}
