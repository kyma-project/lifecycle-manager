package istio_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
)

func Test_NewHTTPRoute_ReturnsCorrectHttpRoute(t *testing.T) {
	watcher := getSimpleWatcher()
	expectedHTTPRouteName := getWatcherName(watcher)
	expectedHTTPRouteMatchURIPrefix := getHTTPRoutePrefix(watcher)
	expectedHTTPRouteDestinationHost := getDestinationHost(watcher)
	expectedHTTPRouteDestinationPort := getDestinationPort(watcher)

	httpRoute := istio.NewHTTPRoute(watcher)

	assert.Equal(t, expectedHTTPRouteName, httpRoute.GetName())
	assert.Equal(t, expectedHTTPRouteMatchURIPrefix, httpRoute.GetMatch()[0].GetUri().GetPrefix())
	assert.Equal(t, expectedHTTPRouteDestinationHost, httpRoute.GetRoute()[0].GetDestination().GetHost())
	assert.Equal(t, expectedHTTPRouteDestinationPort, httpRoute.GetRoute()[0].GetDestination().GetPort().GetNumber())
}

func getWatcherName(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("%v/%v", watcher.Namespace, watcher.Name)
}

func getHTTPRoutePrefix(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("/v1/%s/event", watcher.GetLabels()[shared.ManagedBy])
}

func getDestinationHost(watcher *v1beta2.Watcher) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", watcher.Spec.ServiceInfo.Name, watcher.Spec.ServiceInfo.Namespace)
}

func getDestinationPort(watcher *v1beta2.Watcher) uint32 {
	return uint32(watcher.Spec.ServiceInfo.Port)
}
