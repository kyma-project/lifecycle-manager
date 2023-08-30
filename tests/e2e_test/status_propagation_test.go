//go:build status_propagation_e2e

package e2e_test

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	timeout      = 10 * time.Second
	readyTimeout = 2 * time.Minute
	interval     = 1 * time.Second

	watcherPodContainer = "server"
	exampleSKRDomain    = "example.domain.com"

	KLMPodPrefix    = "klm-controller-manager"
	KLMPodContainer = "manager"

	defaultRuntimeNamespace = "kyma-system"
	controlPlaneNamespace   = "kcp-system"
)

var (
	errPodNotFound               = errors.New("could not find pod")
	errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")
	errLogNotFound               = errors.New("logMsg was not found in log")
	errKymaNotDeleted            = errors.New("kyma CR not deleted")
)

var _ = Describe("Enable Template Operator, Kyma CR should have status `Warning`",
	Ordered, func() {
		channel := "regular"
		kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", channel)
		GinkgoWriter.Printf("kyma before create %v\n", kyma)
		remoteNamespace := "kyma-system"

		BeforeAll(func() {
			// make sure we can list Kymas to ensure CRDs have been installed
			err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
			Expect(meta.IsNoMatchError(err)).To(BeFalse())
		})

		It("Should create empty Kyma CR on remote cluster", func() {
			Eventually(CreateKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(controlPlaneClient.Create, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("verifying kyma is ready")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
			By("verifying remote kyma is ready")
			Eventually(CheckRemoteKymaCR, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
				Should(Succeed())
		})

		It("Should enable Template Operator and Kyma should result in Warning status", func() {
			By("Enabling Template Operator")
			Eventually(enableModule, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), "template-operator", "regular", controlPlaneClient).
				Should(Succeed())
			By("Checking state of kyma")
			Eventually(CheckKymaIsInState, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
				Should(Succeed())
		})

	})

func enableModule(ctx context.Context, kymaName, kymaNamespace, moduleName, moduleChannel string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	GinkgoWriter.Printf("kyma %v\n", kyma)
	kyma.Spec.Modules = append(kyma.Spec.Modules, v1beta2.Module{
		Name:    moduleName,
		Channel: moduleChannel,
	})
	return k8sClient.Update(ctx, kyma)
}
