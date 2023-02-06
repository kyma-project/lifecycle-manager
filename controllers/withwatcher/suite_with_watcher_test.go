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

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"
	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/istio"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"

	//+kubebuilder:scaffold:imports
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/deploy"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	istioscheme "istio.io/client-go/pkg/clientset/versioned/scheme"
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
	webhookChartPath       = "../../skr-webhook"
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
	})
	Expect(err).NotTo(HaveOccurred())

	// kcpModule CRD
	controlplaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(k8syaml.Unmarshal(moduleFile, &controlplaneCrd)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlplaneCrd}, remoteCrds...),
		ErrorIfCRDPathMissing: true,
	}

	restCfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restCfg).NotTo(BeNil())

	Expect(operatorv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(moduleManagerV1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(istioscheme.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	controlPlaneClient, err = client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(controlPlaneClient).NotTo(BeNil())

	runtimeClient, runtimeEnv = NewSKRCluster()

	metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
	if !found {
		metricsBindAddress = ":8081"
	}

	k8sManager, err = ctrl.NewManager(
		restCfg, ctrl.Options{
			MetricsBindAddress: metricsBindAddress,
			Scheme:             scheme.Scheme,
			NewCache:           controllers.NewCacheFunc(),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := controllers.RequeueIntervals{
		Success: 3 * time.Second,
	}

	Expect(createLoadBalancer(suiteCtx, controlPlaneClient)).To(Succeed())
	istioResources, err = deserializeIstioResources()
	Expect(err).NotTo(HaveOccurred())
	for _, istioResource := range istioResources {
		Expect(controlPlaneClient.Create(suiteCtx, istioResource)).To(Succeed())
	}

	remoteClientCache = remote.NewClientCache()
	skrChartCfg := &deploy.SkrChartManagerConfig{
		WebhookChartPath:       webhookChartPath,
		SkrWebhookMemoryLimits: "200Mi",
		SkrWebhookCPULimits:    "1",
	}
	skrWebhookChartManager, err := deploy.NewSKRWebhookTemplateChartManager(restCfg, skrChartCfg)
	Expect(err).ToNot(HaveOccurred())
	err = (&controllers.KymaReconciler{
		Client:                 k8sManager.GetClient(),
		EventRecorder:          k8sManager.GetEventRecorderFor(operatorv1alpha1.OperatorName),
		RequeueIntervals:       intervals,
		SKRWebhookChartManager: skrWebhookChartManager,
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

func NewSKRCluster() (client.Client, *envtest.Environment) {
	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := skrEnv.Start()
	Expect(cfg).NotTo(BeNil())
	Expect(err).NotTo(HaveOccurred())

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	Expect(err).NotTo(HaveOccurred())

	remote.LocalClient = func() *rest.Config {
		return authUser.Config()
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: controlPlaneClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())

	return skrClient, skrEnv
}

func createLoadBalancer(ctx context.Context, k8sClient client.Client) error {
	istioNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy.IstioSystemNs,
		},
	}
	if err := k8sClient.Create(ctx, istioNs); err != nil {
		return err
	}
	loadBalancerService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.IngressServiceName,
			Namespace: deploy.IstioSystemNs,
			Labels: map[string]string{
				"app": deploy.IngressServiceName,
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
		Name:      deploy.IngressServiceName,
		Namespace: deploy.IstioSystemNs,
	}, loadBalancerService)
}
