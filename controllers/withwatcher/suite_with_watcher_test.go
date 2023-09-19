/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
//nolint:gochecknoglobals
package withwatcher_test

import (
	//+kubebuilder:scaffold:imports
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	istioapi "istio.io/api/networking/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	istioscheme "istio.io/client-go/pkg/clientset/versioned/scheme"

	certManagerV1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const listenerAddr = ":8082"

var (
	controlPlaneClient client.Client
	runtimeClient      client.Client
	k8sManager         manager.Manager
	controlPlaneEnv    *envtest.Environment
	runtimeEnv         *envtest.Environment
	suiteCtx           context.Context
	cancel             context.CancelFunc
	restCfg            *rest.Config
	istioResources     []*unstructured.Unstructured
	remoteClientCache  *remote.ClientCache
	logger             logr.Logger
)

const (
	skrWatcherPath         = "../../skr-webhook"
	istioResourcesFilePath = "../../config/samples/tests/istio-test-resources.yaml"
	istioSystemNs          = "istio-system"
	kcpSystemNs            = "kcp-system"
	gatewayName            = "lifecycle-manager-watcher-gateway"
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	suiteCtx, cancel = context.WithCancel(context.TODO())
	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping test environment")

	externalCRDs := AppendExternalCRDs(
		filepath.Join("..", "..", "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")

	kcpModuleCRD := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml.Unmarshal(moduleFile, &kcpModuleCRD)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{kcpModuleCRD}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
	}

	restCfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	Expect(api.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(istioscheme.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(certManagerV1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
	if !found {
		metricsBindAddress = ":0"
	}

	k8sManager, err = ctrl.NewManager(
		restCfg, ctrl.Options{
			MetricsBindAddress: metricsBindAddress,
			Scheme:             scheme.Scheme,
			Cache:              controllers.NewCacheOptions(),
		})
	Expect(err).ToNot(HaveOccurred())

	controlPlaneClient = k8sManager.GetClient()
	runtimeClient, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())

	intervals := controllers.RequeueIntervals{
		Success: 3 * time.Second,
	}

	// This k8sClient is used to install external resources
	k8sClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	Expect(createNamespace(suiteCtx, istioSystemNs, controlPlaneClient)).To(Succeed())
	Expect(createNamespace(suiteCtx, kcpSystemNs, controlPlaneClient)).To(Succeed())

	Expect(createGateway(suiteCtx, restCfg)).To(Succeed())
	istioResources, err = deserializeIstioResources()
	Expect(err).NotTo(HaveOccurred())
	for _, istioResource := range istioResources {
		Expect(k8sClient.Create(suiteCtx, istioResource)).To(Succeed())
	}

	remoteClientCache = remote.NewClientCache()
	skrChartCfg := &watcher.SkrWebhookManagerConfig{
		SKRWatcherPath:         skrWatcherPath,
		SkrWebhookMemoryLimits: "200Mi",
		SkrWebhookCPULimits:    "1",
		IstioNamespace:         istioSystemNs,
		IstioGatewayName:       gatewayName,
		IstioGatewayNamespace:  kcpSystemNs,
		RemoteSyncNamespace:    controllers.DefaultRemoteSyncNamespace,
	}

	skrWebhookChartManager, err := watcher.NewSKRWebhookManifestManager(restCfg, scheme.Scheme, skrChartCfg)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.KymaReconciler{
		Client:            k8sManager.GetClient(),
		EventRecorder:     k8sManager.GetEventRecorderFor(v1beta2.OperatorName),
		RequeueIntervals:  intervals,
		SKRWebhookManager: skrWebhookChartManager,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache:   remoteClientCache,
		KcpRestConfig:       k8sManager.GetConfig(),
		RemoteSyncNamespace: controllers.DefaultRemoteSyncNamespace,
		InKCPMode:           true,
	}).SetupWithManager(k8sManager, controller.Options{}, controllers.SetupUpSetting{ListenerAddr: listenerAddr})
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.WatcherReconciler{
		Client:           k8sManager.GetClient(),
		RestConfig:       k8sManager.GetConfig(),
		EventRecorder:    k8sManager.GetEventRecorderFor(controllers.WatcherControllerName),
		Scheme:           scheme.Scheme,
		RequeueIntervals: intervals,
	}).SetupWithManager(
		k8sManager, controller.Options{
			MaxConcurrentReconciles: 1,
		},
	)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(suiteCtx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	// clean up istio resources
	for _, istioResource := range istioResources {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(controlPlaneClient, istioResource).Should(Succeed())
	}
	// cancel environment context
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	err = runtimeEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createNamespace(ctx context.Context, namespace string, k8sClient client.Client) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	return k8sClient.Create(ctx, ns)
}

func createGateway(ctx context.Context, restConfig *rest.Config) error {
	gateway := &istiov1beta1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: kcpSystemNs,
			Labels: map[string]string{
				"app": gatewayName,
			},
		},
		Spec: istioapi.Gateway{
			Servers: []*istioapi.Server{
				{
					Port: &istioapi.Port{
						Number: 443,
						Name:   "https",
					},
					Hosts: []string{"example.host"},
				},
			},
			Selector: nil,
		},
	}

	ic, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	_, err = ic.NetworkingV1beta1().Gateways(kcpSystemNs).Create(ctx, gateway, metav1.CreateOptions{})

	return err
}
