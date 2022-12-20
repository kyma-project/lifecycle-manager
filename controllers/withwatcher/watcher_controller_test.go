package withwatcher_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func cRSpecsUpdates() func(customIstioClient *istio.Client) {
	return func(customIstioClient *istio.Client) {
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		Expect(watcherCrs).NotTo(BeEmpty())
		for _, watcherCR := range watcherCrs {
			// update spec
			watcherCR.Spec.ServiceInfo.Port = 9090
			watcherCR.Spec.Field = v1alpha1.StatusField
			Expect(controlPlaneClient.Update(suiteCtx, watcherCR)).Should(Succeed())

			// verify
			Eventually(isCrVsConfigured(suiteCtx, customIstioClient, watcherCR)).Should(BeTrue())
		}
	}
}

func oneCRDeleted() func(customIstioClient *istio.Client) {
	return func(customIstioClient *istio.Client) {
		// delete
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		watcherCR := watcherCrs[crToDeleteIdx]
		Expect(controlPlaneClient.Delete(suiteCtx, watcherCR)).To(Succeed())

		Eventually(isCrDeletionFinished(client.ObjectKeyFromObject(watcherCR)), Timeout, Interval).
			Should(BeTrue())
		Eventually(isCrVsConfigured(suiteCtx, customIstioClient, watcherCR)).Should(BeFalse())
	}
}

func allCRsDeleted() func(customIstioClient *istio.Client) {
	return func(customIstioClient *istio.Client) {
		// delete all remaining CRs
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		watcherCRCount := len(watcherCrs)
		Expect(watcherCRCount).To(Equal(len(centralComponents) - 1))
		for _, watcherCr := range watcherCrs {
			Expect(controlPlaneClient.Delete(suiteCtx, watcherCr)).To(Succeed())
		}
		// verify
		Eventually(isCrDeletionFinished(), Timeout, Interval).Should(BeTrue())
		Eventually(isVsRemoved(suiteCtx, customIstioClient)).Should(BeTrue())
	}
}

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *istio.Client
	var err error
	kymaSample := &v1alpha1.Kyma{}
	BeforeAll(func() {
		// create kyma resource
		kymaSample = NewTestKyma("kyma-sample")

		istioCfg := istio.NewConfig(virtualServiceName)
		customIstioClient, err = istio.NewVersionedIstioClient(restCfg, istioCfg,
			k8sManager.GetEventRecorderFor(controllers.WatcherControllerName), ctrl.Log.WithName("istioClient"))
		Expect(err).ToNot(HaveOccurred())
		Expect(controlPlaneClient.Create(suiteCtx, kymaSample)).To(Succeed())
		// create WatcherCRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(controlPlaneClient.Create(suiteCtx, watcherCR)).To(Succeed())

			// verify
			Eventually(isCrVsConfigured(suiteCtx, customIstioClient, watcherCR)).Should(BeTrue())
		}
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(controlPlaneClient.Delete(suiteCtx, kymaSample)).To(Succeed())
	})

	DescribeTable("given watcherCR reconcile loop",
		func(testCase func(customIstioClient *istio.Client)) {
			testCase(customIstioClient)
		},
		[]TableEntry{
			Entry("when watcherCR specs are updated", cRSpecsUpdates()),
			Entry("when one WatcherCR is deleted", oneCRDeleted()),
			Entry("when all WatcherCRs are deleted", allCRsDeleted()),
		})
})
