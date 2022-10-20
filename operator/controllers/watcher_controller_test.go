package controllers_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Watcher reconciler scenarios", Ordered, func() {

	var err error
	var kymaSample *v1alpha1.Kyma
	var watchers []v1alpha1.Watcher
	var customIstioClient *custom.IstioClient
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		customIstioClient, err = custom.NewVersionedIstioClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		for idx, component := range centralComponents {
			watcher := createWatcherCR(component, isEven(idx))
			watchers = append(watchers, *watcher)
			Expect(controlPlaneClient.Create(ctx, watcher)).To(Succeed())
		}

		istioResources, err = deserializeIstioResources()
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Create(ctx, istioResource)).To(Succeed())
		}
	})

	AfterAll(func() {
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Delete(ctx, istioResource)).To(Succeed())
		}
	})

	It("configures the KCP virtual service and installs skr chart when kyma is enqueued", func() {

		kymaSample = deploy.CreateKymaCR("kyma-sample")
		Expect(controlPlaneClient.Create(ctx, kymaSample)).To(Succeed())
		for idx, watcher := range watchers {
			Eventually(watcherCRState(client.ObjectKeyFromObject(&watchers[idx])),
				30*time.Second, interval).Should(Equal(v1alpha1.WatcherStateReady))
			routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, &watcher)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())
			webhookCfg, err := deploy.GetDeployedWebhook(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.IsWebhookConfigured(&watchers[idx], webhookCfg)).To(BeTrue())
		}
	})
	It("configures the KCP virtual service and re-installs skr chart when watcher list is updated", func() {
		//remove one watcher CR
		//enqueue kyma
		kymaSample2 := deploy.CreateKymaCR("kyma-sample-2")
		Expect(controlPlaneClient.Create(ctx, kymaSample2)).To(Succeed())
		for idx, watcher := range watchers {
			Eventually(watcherCRState(client.ObjectKeyFromObject(&watchers[idx])),
				timeout, interval).Should(Equal(v1alpha1.WatcherStateReady))
			routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, &watcher)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())
			webhookCfg, err := deploy.GetDeployedWebhook(ctx, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy.IsWebhookConfigured(&watchers[idx], webhookCfg)).To(BeTrue())
		}
	})
	It("configures the KCP virtual service and removes skr chart when kyma is deleted", func() {
		Expect(controlPlaneClient.Delete(ctx, kymaSample)).To(Succeed())
		for idx, watcher := range watchers {
			Eventually(watcherCRState(client.ObjectKeyFromObject(&watchers[idx])),
				timeout, interval).Should(Equal(v1alpha1.WatcherStateReady))
			routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, &watcher)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())
		}
		Eventually(deploy.IsChartRemoved(ctx, controlPlaneClient), timeout, interval).Should(BeTrue())
	})
})
