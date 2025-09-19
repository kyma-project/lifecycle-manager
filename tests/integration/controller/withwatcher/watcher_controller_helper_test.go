package withwatcher_test

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/istio"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultBufferSize    = 2048
	componentToBeRemoved = "compass"
	componentToBeUpdated = "lifecycle-manager"
)

var (
	centralComponents                             = []string{componentToBeUpdated, componentToBeRemoved}
	errRouteNotFound                              = errors.New("http route is not found")
	errHTTPRoutesEmpty                            = errors.New("empty http routes")
	errRouteConfigMismatch                        = errors.New("http route config mismatch")
	errVirtualServiceHostsNotMatchGateway         = errors.New("virtual service hosts not match with gateway")
	errWatcherExistsAfterDeletion                 = errors.New("watcher CR still exists after deletion")
	errWatcherNotReady                            = errors.New("watcher not ready")
	errVirtualServiceOwnerReferencesNotConfigured = errors.New(
		"virtual service does not include KLM in owner references",
	)
)

func registerDefaultLifecycleForKymaWithWatcher(kyma *v1beta2.Kyma, watcher *v1beta2.Watcher,
	tlsSecret *apicorev1.Secret, issuer *certmanagerv1.Issuer, gatewaySecret *apicorev1.Secret,
) {
	BeforeAll(func() {
		By("Creating watcher CR")
		Expect(kcpClient.Create(ctx, watcher)).To(Succeed())
		By("Creating kyma CR")
		Expect(kcpClient.Create(ctx, kyma)).To(Succeed())
		By("Creating TLS Secret")
		Expect(kcpClient.Create(ctx, tlsSecret)).To(Succeed())
		By("Creating Cert-Manager Issuer")
		Expect(kcpClient.Create(ctx, issuer)).To(Succeed())
		By("Creating CA Certificate")
		Expect(kcpClient.Create(ctx, gatewaySecret)).To(Succeed())
	})

	AfterAll(func() {
		By("Deleting watcher CR")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, watcher).Should(Succeed())
		By("Ensuring watcher CR is properly deleted")
		Eventually(isWatcherCrDeletionFinished, Timeout, Interval).WithArguments(watcher).
			Should(BeTrue())
		By("Deleting Cert-Manager Issuer")
		Expect(kcpClient.Delete(ctx, issuer)).To(Succeed())
		By("Deleting CA Certificate")
		Expect(kcpClient.Delete(ctx, gatewaySecret)).To(Succeed())
	})

	BeforeEach(func() {
		By("asserting only one kyma CR exists")
		kcpKymas := &v1beta2.KymaList{}
		Eventually(kcpClient.List, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpKymas).Should(Succeed())
		Expect(kcpKymas.Items).NotTo(BeEmpty())
		By("get latest kyma CR")
		Eventually(kcpClient.Get, Timeout, Interval).
			WithContext(ctx).
			WithArguments(client.ObjectKeyFromObject(kyma), kyma).Should(Succeed())
		By("get latest watcher CR")
		Eventually(kcpClient.Get, Timeout, Interval).
			WithContext(ctx).
			WithArguments(client.ObjectKeyFromObject(watcher), watcher).Should(Succeed())
		By("get latest TLS secret")
		Eventually(kcpClient.Get, Timeout, Interval).
			WithContext(ctx).
			WithArguments(client.ObjectKeyFromObject(tlsSecret), tlsSecret).Should(Succeed())
	})
}

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
	decoder := machineryaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
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
		TypeMeta: apimetav1.TypeMeta{
			Kind:       string(shared.WatcherKind),
			APIVersion: v1beta2.GroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      managerInstanceName,
			Namespace: ControlPlaneNamespace,
			Labels: map[string]string{
				shared.ManagedBy: managerInstanceName,
			},
		},
		Spec: v1beta2.WatcherSpec{
			ServiceInfo: v1beta2.Service{
				Port:      8082,
				Name:      managerInstanceName + "-svc",
				Namespace: ControlPlaneNamespace,
			},
			LabelsToWatch: map[string]string{
				managerInstanceName + "-watchable": "true",
			},
			ResourceToWatch: v1beta2.WatchableGVR{
				Group:    v1beta2.GroupVersion.Group,
				Version:  v1beta2.GroupVersion.Version,
				Resource: shared.KymaKind.Plural(),
			},
			Field: field,
			Gateway: v1beta2.GatewayConfig{
				LabelSelector: v1beta2.DefaultIstioGatewaySelector(),
			},
		},
	}
}

func createWatcherSecret(kymaObjKey client.ObjectKey) *apicorev1.Secret {
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaObjKey.Name + "-webhook-tls",
			Namespace: istioSystemNs,
			Labels: map[string]string{
				shared.ManagedBy: shared.OperatorName,
			},
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("jelly"),
			"tls.crt": []byte("jellyfish"),
			"tls.key": []byte("jellyfishes"),
		},
		Type: apicorev1.SecretTypeOpaque,
	}
}

func createGatewaySecret() *apicorev1.Secret {
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "klm-istio-gateway",
			Namespace: istioSystemNs,
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: apimetav1.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
		},
		Data: map[string][]byte{
			"ca.crt":  []byte("jelly"),
			"tls.crt": []byte("jellyfish"),
			"tls.key": []byte("jellyfishes"),
		},
		Type: apicorev1.SecretTypeOpaque,
	}
}

func getWatcher(name string) (*v1beta2.Watcher, error) {
	watcherCR := &v1beta2.Watcher{}
	err := kcpClient.Get(ctx,
		client.ObjectKey{Name: name, Namespace: ControlPlaneNamespace},
		watcherCR)
	return watcherCR, err
}

func isVirtualServiceHostsConfigured(ctx context.Context,
	vsName, vsNamespace string,
	istioClient *istio.Client,
	gateway *istioclientapiv1beta1.Gateway,
) error {
	virtualService, err := istioClient.GetVirtualService(ctx, vsName, vsNamespace)
	if err != nil {
		return err
	}
	if !contains(virtualService.Spec.GetHosts(), gateway.Spec.GetServers()[0].GetHosts()[0]) {
		return errVirtualServiceHostsNotMatchGateway
	}
	return nil
}

func verifyWatcherConfiguredAsVirtualServiceOwner(ctx context.Context,
	vsName, vsNamespace string,
	watcher *v1beta2.Watcher,
	istioClient *istio.Client,
) error {
	virtualService, err := istioClient.GetVirtualService(ctx, vsName, vsNamespace)
	if err != nil {
		return err
	}

	watcherInOwnerReferences := false
	for _, ownerReference := range virtualService.GetObjectMeta().GetOwnerReferences() {
		if ownerReference.Name == watcher.GetName() &&
			ownerReference.Kind == watcher.Kind &&
			ownerReference.UID == watcher.GetUID() {
			watcherInOwnerReferences = true
		}
	}

	if !watcherInOwnerReferences {
		return errVirtualServiceOwnerReferencesNotConfigured
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

func isListenerHTTPRouteConfigured(ctx context.Context, clt *istio.Client, namespace string, watcher *v1beta2.Watcher,
) error {
	virtualService, err := clt.GetVirtualService(ctx, watcher.Name, namespace)
	if err != nil {
		return err
	}
	if len(virtualService.Spec.GetHttp()) == 0 {
		return errHTTPRoutesEmpty
	}

	for idx, route := range virtualService.Spec.GetHttp() {
		if route.GetName() == client.ObjectKeyFromObject(watcher).String() {
			istioHTTPRoute, _ := istio.NewHTTPRoute(watcher)
			if !isRouteConfigEqual(virtualService.Spec.GetHttp()[idx], istioHTTPRoute) {
				return errRouteConfigMismatch
			}
			return nil
		}
	}

	return errRouteNotFound
}

func listenerHTTPRouteExists(ctx context.Context, clt *istio.Client, namespace string,
	watcherObjKey client.ObjectKey,
) error {
	virtualService, err := clt.GetVirtualService(ctx, watcherObjKey.Name, namespace)
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

func isRouteConfigEqual(route1 *istioapiv1beta1.HTTPRoute, route2 *istioapiv1beta1.HTTPRoute) bool {
	const firstElementIdx = 0

	stringMatch1, ok := route1.GetMatch()[firstElementIdx].GetUri().GetMatchType().(*istioapiv1beta1.StringMatch_Prefix)
	if !ok {
		return false
	}
	stringMatch2, ok := route2.GetMatch()[firstElementIdx].GetUri().GetMatchType().(*istioapiv1beta1.StringMatch_Prefix)
	if !ok {
		return false
	}

	if stringMatch1.Prefix != stringMatch2.Prefix {
		return false
	}

	if route1.GetRoute()[firstElementIdx].GetDestination().GetHost() !=
		route2.GetRoute()[firstElementIdx].GetDestination().GetHost() {
		return false
	}

	if route1.GetRoute()[firstElementIdx].GetDestination().GetPort().GetNumber() !=
		route2.GetRoute()[firstElementIdx].GetDestination().GetPort().GetNumber() {
		return false
	}

	return true
}
