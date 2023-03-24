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

package withwatcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	certManagerV1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioscheme "istio.io/client-go/pkg/clientset/versioned/scheme"
	corev1 "k8s.io/api/core/v1"
	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
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
	operatorv1beta1 "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	//+kubebuilder:scaffold:imports
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const listenerAddr = ":8082"

var (
	controlPlaneClient client.Client                //nolint:gochecknoglobals
	runtimeClient      client.Client                //nolint:gochecknoglobals
	k8sManager         manager.Manager              //nolint:gochecknoglobals
	controlPlaneEnv    *envtest.Environment         //nolint:gochecknoglobals
	runtimeEnv         *envtest.Environment         //nolint:gochecknoglobals
	suiteCtx           context.Context              //nolint:gochecknoglobals
	cancel             context.CancelFunc           //nolint:gochecknoglobals
	restCfg            *rest.Config                 //nolint:gochecknoglobals
	istioResources     []*unstructured.Unstructured //nolint:gochecknoglobals
	remoteClientCache  *remote.ClientCache          //nolint:gochecknoglobals
	logger             logr.Logger                  //nolint:gochecknoglobals
)

const (
	skrWatcherPath         = "../../skr-webhook"
	istioResourcesFilePath = "../../config/samples/tests/istio-test-resources.yaml"
	virtualServiceName     = "kcp-events"
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

	// manifest CRD
	// istio CRDs
	remoteCrds, err := ParseRemoteCRDs([]string{
		"https://raw.githubusercontent.com/kyma-project/module-manager/main/config/crd/bases/operator.kyma-project.io_manifests.yaml", //nolint:lll
		"https://raw.githubusercontent.com/istio/istio/master/manifests/charts/base/crds/crd-all.gen.yaml",                            //nolint:lll
		"https://github.com/cert-manager/cert-manager/releases/download/v1.10.1/cert-manager.crds.yaml",                               //nolint:lll
	})
	Expect(err).NotTo(HaveOccurred())

	// kcpModule CRD
	controlplaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &controlplaneCrd)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlplaneCrd}, remoteCrds...),
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
			NewCache:           controllers.NewCacheFunc(),
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

	Expect(createLoadBalancer(suiteCtx, k8sClient)).To(Succeed())
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
		IstioNamespace:         metav1.NamespaceDefault,
	}
	skrWebhookChartManager, err := watcher.NewSKRWebhookManifestManager(restCfg, skrChartCfg)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.KymaReconciler{
		Client:            k8sManager.GetClient(),
		EventRecorder:     k8sManager.GetEventRecorderFor(operatorv1beta1.OperatorName),
		RequeueIntervals:  intervals,
		SKRWebhookManager: skrWebhookChartManager,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache: remoteClientCache,
		KcpRestConfig:     k8sManager.GetConfig(),
	}).SetupWithManager(k8sManager, controller.Options{}, controllers.SetupUpSetting{ListenerAddr: listenerAddr})
	Expect(err).ToNot(HaveOccurred())

	istioCfg := istio.NewConfig(virtualServiceName, false)
	err = (&controllers.WatcherReconciler{
		Client:           k8sManager.GetClient(),
		RestConfig:       k8sManager.GetConfig(),
		EventRecorder:    k8sManager.GetEventRecorderFor(controllers.WatcherControllerName),
		Scheme:           scheme.Scheme,
		RequeueIntervals: intervals,
	}).SetupWithManager(
		k8sManager, controller.Options{
			MaxConcurrentReconciles: 1,
		}, istioCfg,
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
		Expect(controlPlaneClient.Delete(suiteCtx, istioResource)).To(Succeed())
	}
	// cancel environment context
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	err = runtimeEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createLoadBalancer(ctx context.Context, k8sClient client.Client) error {
	istioNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: watcher.IstioSystemNs,
		},
	}
	if err := k8sClient.Create(ctx, istioNs); err != nil {
		return err
	}
	loadBalancerService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watcher.IngressServiceName,
			Namespace: watcher.IstioSystemNs,
			Labels: map[string]string{
				"app": watcher.IngressServiceName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "http2",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	if err := k8sClient.Create(ctx, loadBalancerService); err != nil {
		return err
	}
	loadBalancerService.Status = corev1.ServiceStatus{
		LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{
				{
					IP: "10.10.10.167",
				},
			},
		},
	}
	if err := k8sClient.Status().Update(ctx, loadBalancerService); err != nil {
		return err
	}

	return k8sClient.Get(ctx, client.ObjectKey{
		Name:      watcher.IngressServiceName,
		Namespace: watcher.IstioSystemNs,
	}, loadBalancerService)
}
