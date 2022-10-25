package controllers_with_watcher_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers/test_helper"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

func cRSpecsUpdates() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		Expect(watcherCrs).NotTo(BeEmpty())
		for _, watcherCR := range watcherCrs {
			// update spec
			watcherCR.Spec.ServiceInfo.Port = 9090
			watcherCR.Spec.Field = v1alpha1.StatusField
			Expect(controlPlaneClient.Update(ctx, watcherCR)).Should(Succeed())

			// verify
			Eventually(isCrVsConfigured(ctx, customIstioClient, watcherCR)).Should(BeTrue())

		}
	}
}

func oneCRDeleted() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		// delete
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		watcherCR := watcherCrs[crToDeleteIdx]
		Expect(controlPlaneClient.Delete(ctx, watcherCR)).To(Succeed())

		Eventually(isCrDeletionFinished(client.ObjectKeyFromObject(watcherCR)), timeout, interval).
			Should(BeTrue())
		Eventually(isCrVsConfigured(ctx, customIstioClient, watcherCR)).Should(BeFalse())
	}
}

func allCRsDeleted() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		// delete all
		watcherCrs := listTestWatcherCrs(controlPlaneClient)
		watcherCRCount := len(watcherCrs)
		Expect(watcherCRCount).To(Equal(len(centralComponents) - 1))
		for _, watcherCr := range watcherCrs {
			Expect(controlPlaneClient.Delete(ctx, watcherCr)).To(Succeed())
		}
		// verify
		Eventually(isCrDeletionFinished(), timeout, interval).Should(BeTrue())
		Eventually(isVsRemoved(ctx, customIstioClient)).Should(BeTrue())
	}
}

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *custom.IstioClient
	var err error
	kymaSample := &v1alpha1.Kyma{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		// create kyma resource
		kymaSample = test_helper.NewTestKyma("kyma-sample")

		// create istio resources
		customIstioClient, err = custom.NewVersionedIstioClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, kymaSample)).To(Succeed())
		istioResources, err = deserializeIstioResources()
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Create(ctx, istioResource)).To(Succeed())
		}
		// create WatcherCRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(controlPlaneClient.Create(ctx, watcherCR)).To(Succeed())

			// verify
			Eventually(isCrVsConfigured(ctx, customIstioClient, watcherCR)).Should(BeTrue())
		}
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(controlPlaneClient.Delete(ctx, kymaSample)).To(Succeed())
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Delete(ctx, istioResource)).To(Succeed())
		}

	})

	//BeforeEach(func() {
	//	// create WatcherCRs
	//	for idx, component := range centralComponents {
	//		watcherCR := createWatcherCR(component, isEven(idx))
	//		Expect(controlPlaneClient.Create(ctx, watcherCR)).To(Succeed())
	//
	//		// verify
	//		Eventually(isCrVsConfigured(ctx, customIstioClient, watcherCR)).Should(BeTrue())
	//	}
	//})

	//AfterEach(func() {
	//	for idx, component := range centralComponents {
	//		watcherCR := &v1alpha1.Watcher{}
	//		err := controlPlaneClient.Get(ctx, client.ObjectKey{
	//			Name:      fmt.Sprintf("%s-sample", component),
	//			Namespace: metav1.NamespaceDefault,
	//		}, watcherCR)
	//		if apierrors.IsNotFound(err) && idx == crToDeleteIdx {
	//			continue
	//		}
	//		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
	//
	//		Expect(controlPlaneClient.Delete(ctx, watcherCR)).To(Succeed())
	//	}
	//
	//})

	DescribeTable("given watcherCR reconcile loop",
		func(testCase func(customIstioClient *custom.IstioClient)) {
			testCase(customIstioClient)
		},
		[]TableEntry{
			Entry("when watcherCR specs are updated", cRSpecsUpdates()),
			Entry("when one WatcherCR is deleted", oneCRDeleted()),
			Entry("when all WatcherCRs are deleted", allCRsDeleted()),
		})
})
