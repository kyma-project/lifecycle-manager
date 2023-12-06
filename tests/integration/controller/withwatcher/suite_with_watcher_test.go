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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	istioscheme "istio.io/client-go/pkg/clientset/versioned/scheme"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
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
	istioSystemNs     = "istio-system"
	kcpSystemNs       = "kcp-system"
	gatewayName       = "klm-watcher-gateway"
	caCertificateName = "klm-watcher-serving-cert"
)

var (
	skrWatcherPath         = filepath.Join(integration.GetProjectRoot(), "skr-webhook")
	istioResourcesFilePath = filepath.Join(integration.GetProjectRoot(), "config", "samples", "tests",
		"istio-test-resources.yaml")
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	suiteCtx, cancel = context.WithCancel(context.TODO())
	logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

	By("bootstrapping test environment")

	externalCRDs, err := AppendExternalCRDs(
		filepath.Join(integration.GetProjectRoot(), "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")
	Expect(err).ToNot(HaveOccurred())
	kcpModuleCRD := &apiextensionsv1.CustomResourceDefinition{}
	modulePath := filepath.Join(integration.GetProjectRoot(), "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(machineryaml.Unmarshal(moduleFile, &kcpModuleCRD)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		CRDs:                  append([]*apiextensionsv1.CustomResourceDefinition{kcpModuleCRD}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
	}

	restCfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(istioscheme.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(certmanagerv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
	if !found {
		metricsBindAddress = ":0"
	}

	k8sManager, err = ctrl.NewManager(
		restCfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: metricsBindAddress,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache:  controller.NewCacheOptions(),
		})
	Expect(err).ToNot(HaveOccurred())

	controlPlaneClient = k8sManager.GetClient()
	runtimeClient, runtimeEnv, err = NewSKRCluster(controlPlaneClient.Scheme())
	Expect(err).ToNot(HaveOccurred())

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
		Warning: 100 * time.Millisecond,
	}

	// This k8sClient is used to install external resources
	k8sClient, err := client.New(restCfg, client.Options{Scheme: k8sclientscheme.Scheme})
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
	skrChartCfg := watcher.SkrWebhookManagerConfig{
		SKRWatcherPath:         skrWatcherPath,
		SkrWebhookMemoryLimits: "200Mi",
		SkrWebhookCPULimits:    "1",
		RemoteSyncNamespace:    controller.DefaultRemoteSyncNamespace,
	}

	certificateConfig := watcher.CertificateConfig{
		IstioNamespace:      istioSystemNs,
		RemoteSyncNamespace: controller.DefaultRemoteSyncNamespace,
		CACertificateName:   caCertificateName,
		AdditionalDNSNames:  []string{},
		Duration:            1 * time.Hour,
		RenewBefore:         5 * time.Minute,
	}

	gatewayConfig := watcher.GatewayConfig{
		IstioGatewayName:          gatewayName,
		IstioGatewayNamespace:     kcpSystemNs,
		LocalGatewayPortOverwrite: "",
	}

	caCertCache := watcher.NewCACertificateCache(5 * time.Minute)

	skrWebhookChartManager, err := watcher.NewSKRWebhookManifestManager(
		restCfg, k8sclientscheme.Scheme,
		caCertCache,
		skrChartCfg, certificateConfig, gatewayConfig)
	Expect(err).ToNot(HaveOccurred())
	err = (&controller.KymaReconciler{
		Client:            k8sManager.GetClient(),
		EventRecorder:     k8sManager.GetEventRecorderFor(v1beta2.OperatorName),
		RequeueIntervals:  intervals,
		SKRWebhookManager: skrWebhookChartManager,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache:   remoteClientCache,
		KcpRestConfig:       k8sManager.GetConfig(),
		RemoteSyncNamespace: controller.DefaultRemoteSyncNamespace,
		InKCPMode:           true,
		Metrics:             metrics.NewKymaMetrics(),
	}).SetupWithManager(k8sManager, ctrlruntime.Options{}, controller.SetupUpSetting{ListenerAddr: listenerAddr})
	Expect(err).ToNot(HaveOccurred())

	err = (&controller.WatcherReconciler{
		Client:           k8sManager.GetClient(),
		RestConfig:       k8sManager.GetConfig(),
		EventRecorder:    k8sManager.GetEventRecorderFor(controller.WatcherControllerName),
		Scheme:           k8sclientscheme.Scheme,
		RequeueIntervals: intervals,
	}).SetupWithManager(
		k8sManager, ctrlruntime.Options{
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
	ns := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: namespace,
		},
	}
	return k8sClient.Create(ctx, ns)
}

func createGateway(ctx context.Context, restConfig *rest.Config) error {
	gateway := &istioclientapiv1beta1.Gateway{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      gatewayName,
			Namespace: kcpSystemNs,
			Labels: map[string]string{
				"app": gatewayName,
			},
		},
		Spec: istioapiv1beta1.Gateway{
			Servers: []*istioapiv1beta1.Server{
				{
					Port: &istioapiv1beta1.Port{
						Number: 443,
						Name:   "https",
					},
					Hosts: []string{"example.host"},
				},
			},
			Selector: nil,
		},
	}

	ic, err := istioclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	_, err = ic.NetworkingV1beta1().Gateways(kcpSystemNs).Create(ctx, gateway, apimetav1.CreateOptions{})

	return err
}
