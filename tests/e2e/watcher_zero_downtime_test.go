package e2e_test

import (
	"context"
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Watcher Zero Downtime", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When SKR metrics service is exposed", func() {
			Expect(PatchServiceToTypeLoadBalancer(ctx, skrClient,
				"skr-webhook-metrics", "kyma-system")).
				To(Succeed())
			time.Sleep(1 * time.Second)
		})

		It("Then no downtime errors can be observed", func() {
			Consistently(triggerWatcherAndCheckDowntime).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace).
				WithTimeout(4 * time.Minute).
				WithPolling(10 * time.Second).
				Should(Succeed())
		})
	})
})

func triggerWatcherAndCheckDowntime(ctx context.Context, skrClient client.Client,
	kymaName, kymaNamespace string,
) error {
	// Triggering watcher request
	kyma, err := GetKyma(ctx, skrClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	if kyma.Spec.Channel == v1beta2.DefaultChannel {
		kyma.Spec.Channel = FastChannel
	} else {
		kyma.Spec.Channel = v1beta2.DefaultChannel
	}
	err = skrClient.Update(ctx, kyma)
	if err != nil && !strings.Contains(err.Error(),
		"the object has been modified") {
		return err
	}

	time.Sleep(1 * time.Second)

	// Checking that failed KCP error metrics is not increasing
	count, err := GetWatcherFailedKcpTotalMetric(ctx)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return err
	}
	if count > 0 {
		return errors.New("watcher is experiencing downtime")
	}
	return nil
}
