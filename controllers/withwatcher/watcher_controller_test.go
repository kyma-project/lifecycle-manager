package withwatcher_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func cRSpecsUpdates() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		for _, component := range centralComponents {
			watcherCR, err := getWatcher(component)
			if err != nil {
				return err
			}
			watcherCR.Spec.ServiceInfo.Port = 9090
			watcherCR.Spec.Field = v1beta2.StatusField
			if err := controlPlaneClient.Update(suiteCtx, &watcherCR); err != nil {
				return err
			}
		}
		return nil
	}
}

func gatewayUpdated() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		watcher, err := getWatcher(componentToBeUpdated)
		if err != nil {
			return err
		}
		gateways, err := customIstioClient.LookupGateways(suiteCtx, &watcher)
		if err != nil {
			return err
		}
		Expect(gateways).To(HaveLen(1))
		gateway := gateways[0]
		Expect(gateway.Spec.Servers).To(HaveLen(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(HaveLen(1))
		gateway.Spec.Servers[0].Hosts[0] = "listener.updated.kyma.cloud.sap"
		return controlPlaneClient.Update(suiteCtx, gateway)
	}
}

func expectVirtualServiceConfiguredCorrectly() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		for _, component := range centralComponents {
			watcherCR, err := getWatcher(component)
			if err != nil {
				return err
			}
			if err := isVirtualServiceHTTPRouteConfigured(suiteCtx, customIstioClient, &watcherCR); err != nil {
				return err
			}
			gateways, err := customIstioClient.LookupGateways(suiteCtx, &watcherCR)
			if err != nil {
				return err
			}
			Expect(gateways).To(HaveLen(1))
			if err := isVirtualServiceHostsConfigured(suiteCtx, customIstioClient, gateways[0]); err != nil {
				return err
			}
		}
		return nil
	}
}

func deleteOneWatcherCR() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		return deleteWatcher(componentToBeRemoved)
	}
}

func expectVirtualServiceHTTPRouteRemoved() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		watcherCR, err := getWatcher(componentToBeRemoved)
		if !apierrors.IsNotFound(err) {
			return errWatcherNotRemoved
		}
		err = isVirtualServiceHTTPRouteConfigured(suiteCtx, customIstioClient, &watcherCR)
		if !errors.Is(err, errRouteNotExists) {
			return err
		}
		return nil
	}
}

func expectVirtualServiceRemoved() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		return isVirtualServiceRemoved(suiteCtx, customIstioClient)
	}
}

func deleteWatcher(name string) error {
	watcher, err := getWatcher(name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return controlPlaneClient.Delete(suiteCtx, &watcher)
}

func allCRsDeleted() func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		for _, component := range centralComponents {
			err := deleteWatcher(component)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *istio.Client
	var err error
	BeforeAll(func() {
		istioCfg := istio.NewConfig(virtualServiceName, false)
		customIstioClient, err = istio.NewVersionedIstioClient(restCfg, istioCfg,
			k8sManager.GetEventRecorderFor(controllers.WatcherControllerName), ctrl.Log.WithName("istioClient"))
		Expect(err).ToNot(HaveOccurred())
		// create WatcherCRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(controlPlaneClient.Create(suiteCtx, watcherCR)).To(Succeed())

		}
	})

	DescribeTable("Test VirtualService",
		func(givenCondition func(customIstioClient *istio.Client) error,
			expectedBehavior func(customIstioClient *istio.Client) error,
		) {
			Skip("TODO: revisit it after 542 merged")
			Eventually(givenCondition, Timeout, Interval).WithArguments(customIstioClient).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).WithArguments(customIstioClient).Should(Succeed())
		},
		Entry("when watcherCR specs are updated, "+
			"expect VirtualService configured correctly",
			cRSpecsUpdates(),
			expectVirtualServiceConfiguredCorrectly()),
		Entry("when gateway specs are updated, "+
			"expect VirtualService configured correctly",
			gatewayUpdated(),
			expectVirtualServiceConfiguredCorrectly()),
		Entry("when one WatcherCR is deleted, "+
			"expect related VirtualService http route removed",
			deleteOneWatcherCR(),
			expectVirtualServiceHTTPRouteRemoved()),
		Entry("when all WatcherCRs are deleted,"+
			"expect VirtualService removed",
			allCRsDeleted(),
			expectVirtualServiceRemoved()))
})
