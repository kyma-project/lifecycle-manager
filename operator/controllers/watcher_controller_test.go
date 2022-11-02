package controllers_test

import (
	"os"
	"path/filepath"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	. "github.com/kyma-project/lifecycle-manager/operator/internal/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func cRSpecsUpdates() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		watcherList := v1alpha1.WatcherList{}
		Expect(controlPlaneClient.List(ctx, &watcherList)).To(Succeed())
		Expect(watcherList.Items).NotTo(BeEmpty())
		for idx, watcherCR := range watcherList.Items {
			// update spec
			watcherCR.Spec.ServiceInfo.Port = 9090
			watcherCR.Spec.Field = v1alpha1.StatusField
			Expect(controlPlaneClient.Update(ctx, &watcherList.Items[idx])).Should(Succeed())

			// verify
			Eventually(watcherCRState(client.ObjectKeyFromObject(&watcherList.Items[idx])),
				Timeout, Interval).Should(Equal(v1alpha1.WatcherStateReady))
			verifyVsRoutes(&watcherList.Items[idx], customIstioClient, BeTrue())
			webhookCfg, err := deploy.GetDeployedWebhook(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.IsWebhookConfigured(&watcherList.Items[idx], webhookCfg)).To(BeTrue())
		}
	}
}

func oneCRDeleted() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		// delete
		watcherList := v1alpha1.WatcherList{}
		Expect(controlPlaneClient.List(ctx, &watcherList)).To(Succeed())
		watcherCRCount := len(watcherList.Items)
		Expect(watcherCRCount).To(Equal(len(centralComponents)))
		watcherCR := watcherList.Items[watcherCRCount-1]
		Expect(controlPlaneClient.Delete(ctx, &watcherCR)).To(Succeed())

		Eventually(isCrDeletionFinished(client.ObjectKeyFromObject(&watcherCR)), Timeout, Interval).Should(BeTrue())
		verifyVsRoutes(&watcherCR, customIstioClient, BeFalse())
		webhookCfg, err := deploy.GetDeployedWebhook(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(&watcherCR, webhookCfg)).To(BeFalse())
	}
}

func allCRsDeleted() func(customIstioClient *custom.IstioClient) {
	return func(customIstioClient *custom.IstioClient) {
		// delete all
		watcherList := v1alpha1.WatcherList{}
		Expect(controlPlaneClient.List(ctx, &watcherList)).To(Succeed())
		watcherCRCount := len(watcherList.Items)
		Expect(watcherCRCount).To(Equal(len(centralComponents)))
		Expect(controlPlaneClient.DeleteAllOf(ctx, &v1alpha1.Watcher{},
			client.InNamespace(metav1.NamespaceDefault))).To(Succeed())

		// verify
		Eventually(isCrDeletionFinished(), Timeout, Interval).Should(BeTrue())
		verifyVsRoutes(nil, customIstioClient, BeTrue())
		Eventually(IsChartRemoved(ctx, controlPlaneClient), Timeout, Interval).Should(BeTrue())
	}
}

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *custom.IstioClient
	var err error
	kymaSample := &v1alpha1.Kyma{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		// create kyma resource
		kymaName := "kyma-sample"
		kymaSample = CreateKymaCR(kymaName)

		// create istio resources
		customIstioClient, err = custom.NewVersionedIstioClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, kymaSample)).To(Succeed())
		istioResources, err = deserializeIstioResources()
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Create(ctx, istioResource)).To(Succeed())
		}

		Expect(CreateLoadBalancer(ctx, controlPlaneClient)).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(controlPlaneClient.Delete(ctx, kymaSample)).To(Succeed())
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Delete(ctx, istioResource)).To(Succeed())
		}
		// clean rendered manifest
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, RenderedManifestDir))).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		// clean rendered manifest
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, RenderedManifestDir))).ShouldNot(HaveOccurred())
		// create WatcherCRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(controlPlaneClient.Create(ctx, watcherCR)).To(Succeed())
			crObjectKey := client.ObjectKeyFromObject(watcherCR)
			Eventually(watcherCRState(crObjectKey), Timeout, Interval).Should(Equal(v1alpha1.WatcherStateReady))

			// verify
			verifyVsRoutes(watcherCR, customIstioClient, BeTrue())
			webhookCfg, err := deploy.GetDeployedWebhook(ctx, cfg)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(deploy.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
		}
	})

	AfterEach(func() {
		watcherList := v1alpha1.WatcherList{}
		Expect(controlPlaneClient.List(ctx, &watcherList)).To(Succeed())
		for idx := range watcherList.Items {
			// delete WatcherCR
			Expect(controlPlaneClient.Delete(ctx, &watcherList.Items[idx])).To(Succeed())
		}
		// verify deletion
		Eventually(isCrDeletionFinished(), Timeout, Interval).Should(BeTrue())
	})

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
