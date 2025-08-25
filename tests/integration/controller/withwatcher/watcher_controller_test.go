package withwatcher_test

import (
	"errors"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Watcher CR scenarios", Ordered, func() {
	var customIstioClient *istio.Client
	var err error
	BeforeAll(func() {
		customIstioClient, err = istio.NewIstioClient(restCfg, ctrl.Log.WithName("istioClient"))
		Expect(err).ToNot(HaveOccurred())
		// create Watcher CRs
		for idx, component := range centralComponents {
			watcherCR := createWatcherCR(component, isEven(idx))
			Expect(kcpClient.Create(ctx, watcherCR)).To(Succeed())
		}
	})

	It("All created Watcher CRs are in Ready state", func() {
		Eventually(expectWatchersAreReady, Timeout, Interval).
			WithArguments([]string{}).Should(Succeed())
	})

	allVirtualServicesDeleted := allVirtualServicesDeletedForNs(ControlPlaneNamespace)

	DescribeTable("Test VirtualService",
		func(
			timeout, interval time.Duration,
			givenCondition func(istioClt *istio.Client) error,
			expectedVirtualServiceBehavior func(istioClt *istio.Client, namespace string) error,
			expectedWatcherCRBehavior func(watcherNames []string) error,
			watcherNames []string,
		) {
			Eventually(givenCondition, timeout, interval).
				WithArguments(customIstioClient).Should(Succeed())
			Eventually(expectedVirtualServiceBehavior, timeout, interval).
				WithArguments(customIstioClient, ControlPlaneNamespace).Should(Succeed())
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

func crSpecUpdates(_ *istio.Client) error {
	for _, component := range centralComponents {
		watcherCR, err := getWatcher(component)
		if err != nil {
			return err
		}
		watcherCR.Spec.ServiceInfo.Port = 9090
		watcherCR.Spec.Field = v1beta2.StatusField
		err = kcpClient.Update(ctx, watcherCR)
		if err != nil {
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
	gateways, err := customIstioClient.ListGatewaysByLabelSelector(ctx, &watcher.Spec.Gateway.LabelSelector,
		ControlPlaneNamespace)
	if err != nil {
		return err
	}
	Expect(gateways.Items).To(HaveLen(1))
	gateway := gateways.Items[0]
	Expect(gateway.Spec.GetServers()).To(HaveLen(1))
	Expect(gateway.Spec.GetServers()[0].GetHosts()).To(HaveLen(1))
	gateway.Spec.Servers[0].Hosts[0] = "listener.updated.kyma.cloud.sap"
	return kcpClient.Update(ctx, gateway)
}

func expectVirtualServiceConfiguredCorrectly(customIstioClient *istio.Client, namespace string) error {
	for _, component := range centralComponents {
		watcherCR, err := getWatcher(component)
		if err != nil {
			return err
		}
		err = isListenerHTTPRouteConfigured(ctx, customIstioClient, namespace, watcherCR)
		if err != nil {
			return err
		}
		gateways, err := customIstioClient.ListGatewaysByLabelSelector(ctx, &watcherCR.Spec.Gateway.LabelSelector,
			namespace)
		if err != nil {
			return err
		}
		Expect(gateways.Items).To(HaveLen(1))
		err = isVirtualServiceHostsConfigured(ctx, watcherCR.Name, namespace, customIstioClient,
			gateways.Items[0])
		if err != nil {
			return err
		}

		err = verifyWatcherConfiguredAsVirtualServiceOwner(ctx, watcherCR.Name, namespace, watcherCR,
			customIstioClient)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteOneWatcherCR(_ *istio.Client) error {
	return deleteWatcher(componentToBeRemoved)
}

func expectHTTPRouteRemoved(customIstioClient *istio.Client, namespace string) error {
	err := listenerHTTPRouteExists(ctx, customIstioClient, namespace,
		client.ObjectKey{
			Name:      componentToBeRemoved,
			Namespace: apimetav1.NamespaceDefault,
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

func expectVirtualServiceRemoved(customIstioClient *istio.Client, namespace string) error {
	listVirtualServices, err := customIstioClient.ListVirtualServices(ctx, namespace)
	if !util.IsNotFound(err) {
		return err
	}
	if len(listVirtualServices.Items) != 0 {
		return fmt.Errorf("VirtualServiceList is not empty: %d", len(listVirtualServices.Items))
	}
	return nil
}

func deleteWatcher(name string) error {
	watcher, err := getWatcher(name)
	if util.IsNotFound(err) {
		return nil
	}
	return kcpClient.Delete(ctx, watcher)
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

func allVirtualServicesDeletedForNs(namespace string) func(customIstioClient *istio.Client) error {
	return func(customIstioClient *istio.Client) error {
		for _, component := range centralComponents {
			watcherCR, err := getWatcher(component)
			if err != nil {
				return err
			}
			err = customIstioClient.DeleteVirtualService(ctx, watcherCR.GetName(), namespace)
			if err != nil {
				return err
			}
		}
		return nil
	}
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
