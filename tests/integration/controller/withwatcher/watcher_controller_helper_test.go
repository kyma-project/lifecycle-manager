package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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

func registerDefaultLifecycleForKymaWithWatcher(kyma *v1beta2.Kyma, watcher *v1beta2.Watcher,
	tlsSecret *apicorev1.Secret, issuer *certmanagerv1.Issuer, caCert *certmanagerv1.Certificate,
) {
	BeforeAll(func() {
		By("Creating watcher CR")
		Expect(controlPlaneClient.Create(suiteCtx, watcher)).To(Succeed())
		By("Creating kyma CR")
		Expect(controlPlaneClient.Create(suiteCtx, kyma)).To(Succeed())
		By("Creating TLS Secret")
		Expect(controlPlaneClient.Create(suiteCtx, tlsSecret)).To(Succeed())
		By("Creating Cert-Manager Issuer")
		Expect(controlPlaneClient.Create(suiteCtx, issuer)).To(Succeed())
		By("Creating CA Certificate")
		Expect(controlPlaneClient.Create(suiteCtx, caCert)).To(Succeed())
	})

	AfterAll(func() {
		By("Deleting watcher CR")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(controlPlaneClient, watcher).Should(Succeed())
		By("Ensuring watcher CR is properly deleted")
		Eventually(isWatcherCrDeletionFinished, Timeout, Interval).WithArguments(watcher).
			Should(BeTrue())
		By("Deleting Cert-Manager Issuer")
		Expect(controlPlaneClient.Delete(suiteCtx, issuer)).To(Succeed())
		By("Deleting CA Certificate")
		Expect(controlPlaneClient.Delete(suiteCtx, caCert)).To(Succeed())
	})

	BeforeEach(func() {
		By("asserting only one kyma CR exists")
		kcpKymas := &v1beta2.KymaList{}
		Eventually(controlPlaneClient.List, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(kcpKymas).Should(Succeed())
		Expect(kcpKymas.Items).NotTo(BeEmpty())
		By("get latest kyma CR")
		Eventually(controlPlaneClient.Get, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(client.ObjectKeyFromObject(kyma), kyma).Should(Succeed())
		By("get latest watcher CR")
		Eventually(controlPlaneClient.Get, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(client.ObjectKeyFromObject(watcher), watcher).Should(Succeed())
		By("get latest TLS secret")
		Eventually(controlPlaneClient.Get, Timeout, Interval).
			WithContext(suiteCtx).
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

func createCaCertificate() *certmanagerv1.Certificate {
	return &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "klm-watcher-serving-cert",
			Namespace: istioSystemNs,
		},
		Spec: certmanagerv1.CertificateSpec{
			DNSNames:   []string{"listener.kyma.cloud.sap"},
			IsCA:       true,
			CommonName: "klm-watcher-selfsigned-ca",
			SecretName: "klm-watcher-root-secret",
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: map[string]string{
					"operator.kyma-project.io/managed-by": "lifecycle-manager",
				},
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: "RSA",
			},
		},
	}
}

func createWatcherCR(managerInstanceName string, statusOnly bool) *v1beta2.Watcher {
	field := v1beta2.SpecField
	if statusOnly {
		field = v1beta2.StatusField
	}
	return &v1beta2.Watcher{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       string(shared.WatcherKind),
			APIVersion: pkgapiv1beta2.GroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      managerInstanceName,
			Namespace: apimetav1.NamespaceDefault,
			Labels: map[string]string{
				shared.ManagedBy: managerInstanceName,
			},
		},
		Spec: v1beta2.WatcherSpec{
			ServiceInfo: v1beta2.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", managerInstanceName),
				Namespace: apimetav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", managerInstanceName): "true",
			},
			ResourceToWatch: v1beta2.WatchableGVR{
				Group:    pkgapiv1beta2.GroupVersionResource.Group,
				Version:  pkgapiv1beta2.GroupVersionResource.Version,
				Resource: pkgapiv1beta2.GroupVersionResource.Resource,
			},
			Field: field,
			Gateway: v1beta2.GatewayConfig{
				LabelSelector: v1beta2.DefaultIstioGatewaySelector(),
			},
		},
	}
}

func createTLSSecret(kymaObjKey client.ObjectKey) *apicorev1.Secret {
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      watcher.ResolveTLSCertName(kymaObjKey.Name),
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

func getWatcher(name string) (*v1beta2.Watcher, error) {
	watcherCR := &v1beta2.Watcher{}
	err := controlPlaneClient.Get(suiteCtx,
		client.ObjectKey{Name: name, Namespace: apimetav1.NamespaceDefault},
		watcherCR)
	return watcherCR, err
}

func isVirtualServiceHostsConfigured(ctx context.Context,
	vsName string,
	istioClient *istio.Client,
	gateway *istioclientapiv1beta1.Gateway,
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
