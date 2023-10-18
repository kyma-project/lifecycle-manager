package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	istioclientapi "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const (
	defaultBufferSize    = 2048
	componentToBeRemoved = "compass"
	componentToBeUpdated = "lifecycle-manager"
)

//nolint:gochecknoglobals
var (
	centralComponents                     = []string{componentToBeUpdated, componentToBeRemoved}
	errRouteNotFound                      = errors.New("http route is not found")
	errHTTPRoutesEmpty                    = errors.New("empty http routes")
	errRouteConfigMismatch                = errors.New("http route config mismatch")
	errVirtualServiceHostsNotMatchGateway = errors.New("virtual service hosts not match with gateway")
	errWatcherExistsAfterDeletion         = errors.New("watcher CR still exists after deletion")
	errWatcherNotReady                    = errors.New("watcher not ready")
)

func deserializeIstioResources() ([]*unstructured.Unstructured, error) {
	var istioResourcesList []*unstructured.Unstructured

	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer func(file io.ReadCloser) {
		err := file.Close()
		if err != nil {
			logger.Error(err, "failed to close test resources", "path", istioResourcesFilePath)
		}
	}(file)
	decoder := yaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
	for {
		istioResource := &unstructured.Unstructured{}
		err = decoder.Decode(istioResource)
		if err == nil {
			istioResourcesList = append(istioResourcesList, istioResource)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return istioResourcesList, nil
}

func isEven(idx int) bool {
	return idx%2 == 0
}

func createWatcherCR(managerInstanceName string, statusOnly bool) *v1beta2.Watcher {
	field := v1beta2.SpecField
	if statusOnly {
		field = v1beta2.StatusField
	}
	return &v1beta2.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1beta2.WatcherKind),
			APIVersion: v1beta2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      managerInstanceName,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1beta2.ManagedBy: managerInstanceName,
			},
		},
		Spec: v1beta2.WatcherSpec{
			ServiceInfo: v1beta2.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", managerInstanceName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", managerInstanceName): "true",
			},
			ResourceToWatch: v1beta2.WatchableGVR{
				Group:    v1beta2.GroupVersionResource.Group,
				Version:  v1beta2.GroupVersionResource.Version,
				Resource: v1beta2.GroupVersionResource.Resource,
			},
			Field: field,
			Gateway: v1beta2.GatewayConfig{
				LabelSelector: v1beta2.DefaultIstioGatewaySelector(),
			},
		},
	}
}

func createTLSSecret(kymaObjKey client.ObjectKey) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watcher.ResolveTLSCertName(kymaObjKey.Name),
			Namespace: istioSystemNs,
			Labels: map[string]string{
				v1beta2.ManagedBy: v1beta2.OperatorName,
			},
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("jelly"),
			"tls.crt": []byte("jellyfish"),
			"tls.key": []byte("jellyfishes"),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func getWatcher(name string) (*v1beta2.Watcher, error) {
	watcherCR := &v1beta2.Watcher{}
	err := controlPlaneClient.Get(suiteCtx,
		client.ObjectKey{Name: name, Namespace: metav1.NamespaceDefault},
		watcherCR)
	return watcherCR, err
}

func isVirtualServiceHostsConfigured(ctx context.Context,
	vsName string,
	istioClient *istio.Client,
	gateway *istioclientapi.Gateway,
) error {
	virtualService, err := istioClient.GetVirtualService(ctx, vsName)
	if err != nil {
		return err
	}
	if !contains(virtualService.Spec.GetHosts(), gateway.Spec.GetServers()[0].GetHosts()[0]) {
		return errVirtualServiceHostsNotMatchGateway
	}
	return nil
}

func contains(source []string, target string) bool {
	for _, item := range source {
		if item == target {
			return true
		}
	}
	return false
}

func isListenerHTTPRouteConfigured(ctx context.Context, clt *istio.Client, watcher *v1beta2.Watcher,
) error {
	virtualService, err := clt.GetVirtualService(ctx, watcher.Name)
	if err != nil {
		return err
	}
	if len(virtualService.Spec.GetHttp()) == 0 {
		return errHTTPRoutesEmpty
	}

	for idx, route := range virtualService.Spec.GetHttp() {
		if route.GetName() == client.ObjectKeyFromObject(watcher).String() {
			istioHTTPRoute := istio.PrepareIstioHTTPRouteForCR(watcher)
			if !istio.IsRouteConfigEqual(virtualService.Spec.GetHttp()[idx], istioHTTPRoute) {
				return errRouteConfigMismatch
			}
			return nil
		}
	}

	return errRouteNotFound
}

func listenerHTTPRouteExists(ctx context.Context, clt *istio.Client, watcherObjKey client.ObjectKey) error {
	virtualService, err := clt.GetVirtualService(ctx, watcherObjKey.Name)
	if err != nil {
		return err
	}
	if len(virtualService.Spec.GetHttp()) == 0 {
		return errHTTPRoutesEmpty
	}

	for _, route := range virtualService.Spec.GetHttp() {
		if route.GetName() == watcherObjKey.String() {
			return nil
		}
	}

	return errRouteNotFound
}
