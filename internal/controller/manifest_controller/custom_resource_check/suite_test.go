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
package custom_resource_check_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	"go.uber.org/zap/zapcore"
	apicore "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv    *envtest.Environment
	k8sManager ctrl.Manager
	reconciler *declarative.Reconciler
	cfg        *rest.Config
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

var _ = BeforeSuite(
	func() {
		manifesttest.ManifestFilePath = "../../../../pkg/test_samples/oci/rendered.yaml"
		manifesttest.Ctx, manifesttest.Cancel = context.WithCancel(context.TODO())
		logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

		// create registry and server
		newReg := registry.New()
		manifesttest.Server = httptest.NewServer(newReg)

		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: false,
		}

		var err error
		cfg, err = testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		//+kubebuilder:scaffold:scheme

		Expect(api.AddToScheme(k8sclientscheme.Scheme)).To(Succeed())
		Expect(apicore.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

		metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
		if !found {
			metricsBindAddress = ":0"
		}
		cacheOpts := internal.GetCacheOptions(k8slabels.Set{v1beta2.ManagedBy: v1beta2.OperatorName})
		syncPeriod := 2 * time.Second
		cacheOpts.SyncPeriod = &syncPeriod

		k8sManager, err = ctrl.NewManager(
			cfg, ctrl.Options{
				Metrics: metricsserver.Options{
					BindAddress: metricsBindAddress,
				},
				Scheme: k8sclientscheme.Scheme,
				Cache:  cacheOpts,
			},
		)

		k8sManager.GetControllerOptions()
		Expect(err).ToNot(HaveOccurred())

		authUser, err := testEnv.AddUser(
			envtest.User{
				Name:   "skr-admin-account",
				Groups: []string{"system:masters"},
			}, cfg,
		)
		Expect(err).NotTo(HaveOccurred())

		manifesttest.K8sClient = k8sManager.GetClient()

		kcp := &declarative.ClusterInfo{Config: cfg, Client: manifesttest.K8sClient}
		reconciler = declarative.NewFromManager(
			k8sManager, &v1beta2.Manifest{},
			declarative.WithSpecResolver(
				manifest.NewSpecResolver(kcp),
			),
			declarative.WithPermanentConsistencyCheck(true),
			declarative.WithRemoteTargetCluster(
				func(_ context.Context, _ declarative.Object) (*declarative.ClusterInfo, error) {
					return &declarative.ClusterInfo{Config: authUser.Config()}, nil
				},
			),
			manifest.WithClientCacheKey(),
			declarative.WithPostRun{manifest.PostRunCreateCR},
			declarative.WithPreDelete{manifest.PreDeleteDeleteCR},
			declarative.WithCustomReadyCheck(manifest.NewCustomResourceReadyCheck()),
		)

		err = ctrl.NewControllerManagedBy(k8sManager).
			For(&v1beta2.Manifest{}).
			Watches(&apicore.Secret{}, handler.Funcs{}).
			WithOptions(
				ctrlruntime.Options{
					RateLimiter: internal.ManifestRateLimiter(
						1*time.Second, 5*time.Second,
						30, 200,
					),
					MaxConcurrentReconciles: 1,
				},
			).Complete(reconciler)
		Expect(err).ToNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			err = k8sManager.Start(manifesttest.Ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}()
	},
)

var _ = AfterSuite(
	func() {
		manifesttest.Cancel()
		By("tearing down the test environment")
		manifesttest.Server.Close()
		Eventually(func() error { return testEnv.Stop() }, standardTimeout, standardInterval).Should(Succeed())
	},
)
