package controllers_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/custom"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *custom.IstioClient
	var err error
	kymaSample := &v1alpha1.Kyma{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		customIstioClient, err = custom.NewVersionedIstioClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(controlPlaneClient.Create(ctx, kymaSample)).To(Succeed())
		istioResources, err = deserializeIstioResources()
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(controlPlaneClient.Create(ctx, istioResource)).To(Succeed())
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

	DescribeTable("should reconcile istio service mesh resources according to watcher CR config",
		func(watcherCR *v1alpha1.Watcher) {
			// create watcher CR
			Expect(controlPlaneClient.Create(ctx, watcherCR)).To(Succeed())

			time.Sleep(250 * time.Millisecond)
			crObjectKey := client.ObjectKeyFromObject(watcherCR)

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(v1alpha1.WatcherStateReady))

			// verify istio config
			routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
				Name:      vsName,
				Namespace: vsNamespace,
			}, watcherCR)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())

			// verify webhook config
			Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
			Expect(deploy.IsWebhookConfigured(ctx, watcherCR, cfg)).To(BeTrue())

			// update watcher CR spec
			currentWatcherCR := &v1alpha1.Watcher{}
			Expect(controlPlaneClient.Get(ctx, crObjectKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.Spec.ServiceInfo.Port = 9090
			currentWatcherCR.Spec.Field = v1alpha1.StatusField
			Expect(controlPlaneClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			time.Sleep(250 * time.Millisecond)

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(v1alpha1.WatcherStateReady))

			routeReady, err = customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
				Name:      vsName,
				Namespace: vsNamespace,
			}, currentWatcherCR)
			Expect(err).ToNot(HaveOccurred())
			Expect(routeReady).To(BeTrue())

			// verify webhook config
			Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
			Expect(deploy.IsWebhookConfigured(ctx, currentWatcherCR, cfg)).To(BeTrue())
		}, createTableEntries([]string{"lifecycle-manager", "module-manager", "compass"}))

	It("should delete service mesh routes and SKR config when one CR is deleted", func() {
		firstToBeRemovedObjKey := client.ObjectKey{
			Name:      "compass-sample",
			Namespace: metav1.NamespaceDefault,
		}
		firstToBeRemoved := &v1alpha1.Watcher{}
		Expect(controlPlaneClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)).To(Succeed())
		Expect(controlPlaneClient.Delete(ctx, firstToBeRemoved)).To(Succeed())

		time.Sleep(250 * time.Millisecond)

		Eventually(isCrDeletionFinished(firstToBeRemovedObjKey)).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		routeReady, err := customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		}, firstToBeRemoved)
		Expect(err).ToNot(HaveOccurred())
		Expect(routeReady).To(BeFalse())

		// verify webhook config
		Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
		Expect(deploy.IsWebhookConfigured(ctx, firstToBeRemoved, cfg)).To(BeFalse())
	})

	It("should delete all resources on SKR when all CRs are deleted", func() {
		watchers := &v1alpha1.WatcherList{}
		Expect(controlPlaneClient.List(ctx, watchers)).To(Succeed())
		Expect(len(watchers.Items)).To(Equal(2))
		for _, watcher := range watchers.Items {
			//nolint:gosec
			Expect(controlPlaneClient.Delete(ctx, &watcher)).To(Succeed())
		}

		time.Sleep(250 * time.Millisecond)

		Eventually(isCrDeletionFinished()).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())
		Expect(customIstioClient.IsListenerHTTPRoutesEmpty(ctx, client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		})).To(BeTrue())

		routesReady, err := customIstioClient.IsListenerHTTPRoutesEmpty(ctx, client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(routesReady).To(BeTrue())

		Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeFalse())
	})
})
