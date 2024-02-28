package istio_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"
)

func Test_NewVirtualServiceService_ReturnsError_WhenSchemeIsNil(t *testing.T) {
	vss, err := istio.NewVirtualServiceService(nil)

	assert.Nil(t, vss)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "scheme")
}

func Test_NewVirtualServiceService_ReturnsVirtualServiceService(t *testing.T) {
	scheme := setupScheme(t)

	vss, err := istio.NewVirtualServiceService(scheme)

	assert.NotNil(t, vss)
	require.NoError(t, err)
}

func Test_NewVirtualService_ReturnsError_WhenWatcherIsNil(t *testing.T) {
	vss := setupVirtualServiceService(t)
	var watcher *v1beta2.Watcher = nil
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher")
}

func Test_NewVirtualService_ReturnsError_WhenWatcherNameIsEmpty(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := &v1beta2.Watcher{}
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "watcher.Name")
}

func Test_NewVirtualService_SetsCorrectName(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	assert.Equal(t, watcher.Name, vs.Name)
}

func Test_NewVirtualService_ReturnsError_WhenNamespaceIsEmpty(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := ""
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "namespace")
}

func Test_NewVirtualService_SetsCorrectNamespace(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	assert.Equal(t, namespace, vs.Namespace)
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysIsNil(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	var gateways *istioclientapiv1beta1.GatewayList = nil

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateways")
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysAreEmpty(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := &istioclientapiv1beta1.GatewayList{}

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateways")
}

func Test_NewVirtualService_SetsCorrectGateways(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	expectedGatewayNames := getGatewayNamesMap(gateways.Items)

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetGateways(), len(expectedGatewayNames))
	for _, gateway := range vs.Spec.GetGateways() {
		_, exists := expectedGatewayNames[gateway]
		assert.True(t, exists)
	}
}

func Test_NewVirtualService_ReturnsError_WhenGatewaysHaveNoServers(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	for _, gateway := range gateways.Items {
		gateway.Spec.Servers = []*istioapiv1beta1.Server{}
	}

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	require.ErrorIs(t, err, istio.ErrCantFindGatewayServersHost)
}

func Test_NewVirtualService_ReturnsError_WhenGatewayServersHaveNoHosts(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	for _, gateway := range gateways.Items {
		for _, server := range gateway.Spec.GetServers() {
			server.Hosts = []string{}
		}
	}

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	require.ErrorIs(t, err, istio.ErrCantFindGatewayServersHost)
}

func Test_NewVirtualService_SetsCorrectHosts(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	expectedHosts := getHostNamesMap(gateways.Items)

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetHosts(), len(expectedHosts))
	for _, host := range vs.Spec.GetHosts() {
		_, exists := expectedHosts[host]
		assert.True(t, exists)
	}
}

func Test_NewVirtualService_ReturnsError_WhenUnableToCreateHTTPRoute(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	watcher.Spec.ServiceInfo = v1beta2.Service{}

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	assert.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "unable to construct httpRoute from watcher")
}

func Test_NewVirtualService_SetsAHttpRoute(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	assert.Len(t, vs.Spec.GetHttp(), 1)
}

func Test_NewVirtualService_ReturnsError_WhenFailingToAddOwnerReference(t *testing.T) {
	vss, _ := istio.NewVirtualServiceService(machineryruntime.NewScheme()) // does not provide needed Watcher scheme
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.Nil(t, vs)
	require.ErrorIs(t, err, istio.ErrFailedToAddOwnerReference)
}

func Test_NewVirtualService_SetsOwnerReference(t *testing.T) {
	vss := setupVirtualServiceService(t)
	watcher := setupWatcher()
	namespace := setupNamespace()
	gateways := setupGateways()
	expectedOwnerReference := getOwnerReference(watcher)

	vs, err := vss.NewVirtualService(namespace, watcher, gateways)

	require.NoError(t, err)
	ownerReferences := vs.ObjectMeta.GetOwnerReferences()
	assert.Len(t, ownerReferences, 1)
	assert.Equal(t, expectedOwnerReference, ownerReferences[0])
}

func setupVirtualServiceService(t *testing.T) *istio.VirtualServiceService {
	t.Helper()

	vss, _ := istio.NewVirtualServiceService(setupScheme(t))
	return vss
}

func setupScheme(t *testing.T) *machineryruntime.Scheme {
	t.Helper()

	scheme := machineryruntime.NewScheme()
	if err := api.AddToScheme(scheme); err != nil {
		assert.Fail(t, "failed to setup scheme")
	}

	return scheme
}

func setupWatcher() *v1beta2.Watcher {
	watcher := &v1beta2.Watcher{}

	watcher.SetName("watcher-name")
	watcher.SetNamespace(setupNamespace())

	watcher.Labels = map[string]string{
		shared.ManagedBy: "some-manager",
	}

	watcher.Spec.ServiceInfo = v1beta2.Service{
		Name:      "service-name",
		Namespace: setupNamespace(),
		Port:      4711,
	}

	watcher.APIVersion = "operator.kyma-project.io/v1beta2"
	watcher.Kind = "Watcher"
	watcher.UID = uuid.NewUUID()

	return watcher
}

func setupNamespace() string {
	return "bar"
}

func setupGateways() *istioclientapiv1beta1.GatewayList {
	gateways := []*istioclientapiv1beta1.Gateway{}
	for gatewayIndex := 0; gatewayIndex < 3; gatewayIndex++ {
		gateway := &istioclientapiv1beta1.Gateway{}
		gateway.SetName(fmt.Sprintf("gateway-name-%v", gatewayIndex))
		gateway.SetNamespace(setupNamespace())

		servers := []*istioapiv1beta1.Server{}
		for serverIndex := 0; serverIndex < 3; serverIndex++ {
			server := istioapiv1beta1.Server{}

			hosts := []string{}
			for hostIndex := 0; hostIndex < 3; hostIndex++ {
				hosts = append(hosts, fmt.Sprintf("%v-%v-%v.localhost", gatewayIndex, serverIndex, hostIndex))
			}
			server.Hosts = hosts

			servers = append(servers, &server)
		}
		gateway.Spec.Servers = servers

		gateways = append(gateways, gateway)
	}

	return &istioclientapiv1beta1.GatewayList{
		Items: gateways,
	}
}

func getGatewayNamesMap(gateways []*istioclientapiv1beta1.Gateway) map[string]bool {
	gatewayMap := map[string]bool{}

	for _, gateway := range gateways {
		gatewayMap[getGatewayName(gateway)] = true
	}

	return gatewayMap
}

func getHostNamesMap(gateways []*istioclientapiv1beta1.Gateway) map[string]bool {
	hostsMap := map[string]bool{}

	for _, gateway := range gateways {
		for _, server := range gateway.Spec.GetServers() {
			for _, host := range server.GetHosts() {
				hostsMap[host] = true
			}
		}
	}

	return hostsMap
}

func getGatewayName(gateway *istioclientapiv1beta1.Gateway) string {
	return fmt.Sprintf("%v/%v", gateway.Namespace, gateway.Name)
}

func getOwnerReference(watcher *v1beta2.Watcher) apimetav1.OwnerReference {
	return apimetav1.OwnerReference{
		APIVersion: watcher.APIVersion,
		Kind:       watcher.Kind,
		Name:       watcher.GetName(),
		UID:        watcher.GetUID(),
	}
}
