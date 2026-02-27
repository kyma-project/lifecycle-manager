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
package oci_reg_secret_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/google/go-containerregistry/pkg/registry"
	"go.uber.org/zap/zapcore"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
	skrclientcache "github.com/kyma-project/lifecycle-manager/internal/service/skrclient/cache"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	testskrcontext "github.com/kyma-project/lifecycle-manager/tests/integration/commontestutils/skrcontextimpl"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv          *envtest.Environment
	mgr              ctrl.Manager
	reconciler       *declarativev2.Reconciler
	cfg              *rest.Config
	ctx              context.Context
	cancel           context.CancelFunc
	kcpClient        client.Client
	server           *httptest.Server
	manifestFilePath string
)

const (
	standardTimeout  = 30 * time.Second
	standardInterval = 400 * time.Millisecond
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	manifestFilePath = filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples", "oci",
		"rendered.yaml")
	logr := log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter))
	logf.SetLogger(logr)

	// create registry and server
	newReg := registry.New()
	server = httptest.NewServer(newReg)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// +kubebuilder:scaffold:scheme

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).To(Succeed())
	Expect(apicorev1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(certmanagerv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
	if !found {
		metricsBindAddress = ":0"
	}

	mgr, err = ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: metricsBindAddress,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache:  setup.NewDefaultCacheOptions().GetCacheOptions(),
		},
	)
	Expect(err).ToNot(HaveOccurred())

	kcpClient = mgr.GetClient()
	nonExistingSecretName := types.NamespacedName{Namespace: "kcp-system", Name: "non-existing-secret"}
	keyChainLookup := keychainprovider.NewFromSecretKeyChainProvider(kcpClient, nonExistingSecretName)
	extractor := img.NewPathExtractor()
	testEventRec := event.NewRecorderWrapper(mgr.GetEventRecorderFor(shared.OperatorName))
	manifestClient := manifestclient.NewManifestClient(testEventRec, kcpClient)
	orphanDetectionClient := kymarepo.NewRepository(kcpClient, shared.DefaultControlPlaneNamespace)
	orphanDetectionService := orphan.NewDetectionService(orphanDetectionClient)
	accessManagerService := testskrcontext.NewFakeAccessManagerService(testEnv, cfg)
	cachedManifestParser := declarativev2.NewInMemoryCachedManifestParser(declarativev2.DefaultInMemoryParseTTL)

	rateLimiter := internal.RateLimiter(1*time.Second, 5*time.Second, 30, 200)
	reconciler = declarativev2.NewReconciler(queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    1 * time.Second,
		Error:   1 * time.Second,
		Warning: 1 * time.Second,
	},
		rateLimiter,
		metrics.NewManifestMetrics(metrics.NewSharedMetrics()),
		metrics.NewMandatoryModulesMetrics(),
		manifestClient,
		orphanDetectionService,
		spec.NewResolver(keyChainLookup, extractor),
		skrclientcache.NewService(),
		skrclient.NewService(mgr.GetConfig().QPS, mgr.GetConfig().Burst, accessManagerService),
		kcpClient,
		cachedManifestParser,
		declarativev2.NewExistsStateCheck(),
		"",
	)

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Watches(&apicorev1.Secret{}, handler.Funcs{}).
		WithOptions(
			ctrlruntime.Options{
				RateLimiter:             rateLimiter,
				MaxConcurrentReconciles: 1,
			},
		).Complete(reconciler)
	Expect(err).ToNot(HaveOccurred())

	Eventually(CreateNamespace, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, ControlPlaneNamespace).Should(Succeed())
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
},
)

var _ = AfterSuite(
	func() {
		By("tearing down the test environment")
		cancel()
		server.Close()
		Eventually(func() error { return testEnv.Stop() }, standardTimeout, standardInterval).Should(Succeed())
	},
)
