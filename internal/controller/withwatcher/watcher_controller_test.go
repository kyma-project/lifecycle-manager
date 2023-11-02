package withwatcher_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/controller"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func crSpecUpdates(_ *istio.Client) error {
	for _, component := range centralComponents {
		watcherCR, err := getWatcher(component)
		if err != nil {
			return err
		}
		watcherCR.Spec.ServiceInfo.Port = 9090
		watcherCR.Spec.Field = v1beta2.StatusField
		if err := controlPlaneClient.Update(suiteCtx, watcherCR); err != nil {
			return err
		}
	}
	return nil
}

func gatewayUpdated(customIstioClient *istio.Client) error {
	watcher, err := getWatcher(componentToBeUpdated)
	if err != nil {
		return err
	}
	gateways, err := customIstioClient.LookupGateways(suiteCtx, watcher)
	if err != nil {
		return err
	}
	Expect(gateways).To(HaveLen(1))
	gateway := gateways[0]
	Expect(gateway.Spec.GetServers()).To(HaveLen(1))
	Expect(gateway.Spec.GetServers()[0].GetHosts()).To(HaveLen(1))
	gateway.Spec.Servers[0].Hosts[0] = "listener.updated.kyma.cloud.sap"
	return controlPlaneClient.Update(suiteCtx, gateway)
}

func expectVirtualServiceConfiguredCorrectly(customIstioClient *istio.Client) error {
	for _, component := range centralComponents {
		watcherCR, err := getWatcher(component)
		if err != nil {
			return err
		}
		if err := isListenerHTTPRouteConfigured(suiteCtx, customIstioClient, watcherCR); err != nil {
			return err
		}
		gateways, err := customIstioClient.LookupGateways(suiteCtx, watcherCR)
		if err != nil {
			return err
		}
		Expect(gateways).To(HaveLen(1))
		if err := isVirtualServiceHostsConfigured(suiteCtx, watcherCR.Name, customIstioClient, gateways[0]); err != nil {
			return err
		}
	}
	return nil
}

func deleteOneWatcherCR(_ *istio.Client) error {
	return deleteWatcher(componentToBeRemoved)
}

func expectHTTPRouteRemoved(customIstioClient *istio.Client) error {
	err := listenerHTTPRouteExists(suiteCtx, customIstioClient,
		client.ObjectKey{
			Name:      componentToBeRemoved,
			Namespace: metav1.NamespaceDefault,
		})
	if !errors.Is(err, errRouteNotFound) {
		return err
	}
	return nil
}

func expectWatcherCRRemoved(watcherName string) error {
	_, err := getWatcher(watcherName)
	if !util.IsNotFound(err) {
		return errWatcherExistsAfterDeletion
	}
	return nil
}

func expectVirtualServiceRemoved(customIstioClient *istio.Client) error {
	listVirtualServices, err := customIstioClient.ListVirtualServices(suiteCtx)
	if !util.IsNotFound(err) {
		return err
	}
	if len(listVirtualServices.Items) != 0 {
		return fmt.Errorf("VirtualServiceList is not empty: %d", len(listVirtualServices.Items)) //nolint:goerr113
	}
	return nil
}

func deleteWatcher(name string) error {
	watcher, err := getWatcher(name)
	if util.IsNotFound(err) {
		return nil
	}
	return controlPlaneClient.Delete(suiteCtx, watcher)
}

func allCRsDeleted(_ *istio.Client) error {
	for _, component := range centralComponents {
		err := deleteWatcher(component)
		if err != nil {
			return err
		}
	}
	return nil
}

func allVirtualServicesDeleted(customIstioClient *istio.Client) error {
	for _, component := range centralComponents {
		watcherCR, err := getWatcher(component)
		if err != nil {
			return err
		}
		err = customIstioClient.RemoveVirtualServiceForCR(suiteCtx, client.ObjectKeyFromObject(watcherCR))
		if err != nil {
			return err
		}
	}
	return nil
}

func watcherCRIsReady(watcherName string) error {
	watcher, err := getWatcher(watcherName)
	if err != nil {
		return err
	}
	if watcher.Status.State != shared.StateReady {
		return errWatcherNotReady
	}
	return nil
}

func expectWatchersAreReady(watcherNames []string) error {
	if len(watcherNames) == 0 {
		watcherNames = centralComponents
	}
	for _, component := range watcherNames {
		err := watcherCRIsReady(component)
		if err != nil {
			return err
		}
	}
	return nil
}

func expectWatchersDeleted(watcherNames []string) error {
	if len(watcherNames) == 0 {
		watcherNames = centralComponents
	}
	for _, component := range watcherNames {
		err := expectWatcherCRRemoved(component)
		if err != nil {
			return err
		}
	}
	return nil
}

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *istio.Client
	var err error
	BeforeAll(func() {
		customIstioClient, err = istio.NewVersionedIstioClient(restCfg,
			k8sManager.GetEventRecorderFor(controller.WatcherControllerName), ctrl.Log.WithName("istioClient"))
		Expect(err).ToNot(HaveOccurred())
		// create Watcher CRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(controlPlaneClient.Create(suiteCtx, watcherCR)).To(Succeed())
		}
	})

	It("All created Watcher CRs are in Ready state", func() {
		Eventually(expectWatchersAreReady, Timeout, Interval).
			WithArguments([]string{}).Should(Succeed())
	})

	DescribeTable("Test VirtualService",
		func(
			timeout, interval time.Duration,
			givenCondition func(istioClt *istio.Client) error,
			expectedVirtualServiceBehavior func(istioClt *istio.Client) error,
			expectedWatcherCRBehavior func(watcherNames []string) error,
			watcherNames []string,
		) {
			Eventually(givenCondition, timeout, interval).
				WithArguments(customIstioClient).Should(Succeed())
			Eventually(expectedVirtualServiceBehavior, timeout, interval).
				WithArguments(customIstioClient).Should(Succeed())
			Eventually(expectedWatcherCRBehavior, timeout, interval).
				WithArguments(watcherNames).Should(Succeed())
		},
		Entry("when watcherCR specs are updated, "+
			"expect VirtualService configured correctly",
			Timeout,
			Interval,
			crSpecUpdates,
			expectVirtualServiceConfiguredCorrectly,
			expectWatchersAreReady,
			nil,
		),
		Entry("when gateway specs are updated, "+
			"expect VirtualService configured correctly",
			Timeout,
			Interval,
			gatewayUpdated,
			expectVirtualServiceConfiguredCorrectly,
			expectWatchersAreReady,
			nil,
		),
		Entry("when all VirtualServices are deleted, "+
			"expect VirtualServices recreated",
			Timeout,
			Interval,
			allVirtualServicesDeleted,
			expectVirtualServiceConfiguredCorrectly,
			expectWatchersAreReady,
			nil,
		),
		Entry("when one WatcherCR is deleted, "+
			"expect related VirtualService http route removed"+
			"and watcher finalizer is removed",
			Timeout,
			Interval,
			deleteOneWatcherCR,
			expectHTTPRouteRemoved,
			expectWatchersDeleted,
			[]string{componentToBeRemoved},
		),
		Entry("when all WatcherCRs are deleted,"+
			"expect VirtualService removed",
			Timeout,
			Interval,
			allCRsDeleted,
			expectVirtualServiceRemoved,
			expectWatchersDeleted,
			nil,
		),
	)
})
