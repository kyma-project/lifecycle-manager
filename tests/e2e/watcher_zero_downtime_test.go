package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
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
				WithTimeout(12 * time.Minute).
				WithPolling(1 * time.Minute).
				Should(Succeed())
		})
	})
})

func triggerWatcherAndCheckDowntime(ctx context.Context, skrClient client.Client,
	kymaName, kymaNamespace string,
) error {
	if err := forceRotateGCMCert(ctx); err != nil {
		return err
	}

	// Sleep [0,45] seconds to verify at a random point during the rotation.
	// 45 seconds to finish the circle before next polling (60 seconds).
	// Must be greater than 30 seconds (server cert switch grace interval) to catch
	// both, events before and after server cert switch.
	time.Sleep(time.Duration(rand.Intn(46)) * time.Second) //nolint:gosec // non-security jitter for test pacing

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
	if err := skrClient.Update(ctx, kyma); err != nil && !strings.Contains(err.Error(),
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

// For GCM the minimum rotation interval is 5m.
// This is too infrequent for proper testing => force the rotation
func forceRotateGCMCert(ctx context.Context) error {
	cert := &gcertv1alpha1.Certificate{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Name:      shared.CACertificateName,
		Namespace: shared.IstioNamespace,
	}, cert); err != nil {
		if meta.IsNoMatchError(err) {
			return nil // not a GCM cluster; cert-manager rotates on its own schedule
		}
		return fmt.Errorf("failed to get serving cert: %w", err)
	}
	base := cert.DeepCopy()

	cert.Spec.EnsureRenewedAfter = nil
	truePtr := true
	cert.Spec.Renew = &truePtr

	if err := kcpClient.Patch(ctx, cert, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("failed to bump serving cert annotation: %w", err)
	}
	return nil
}
