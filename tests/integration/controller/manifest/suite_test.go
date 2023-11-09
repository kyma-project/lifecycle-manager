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
package manifest_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
	"go.uber.org/zap/zapcore"
	apicorev1 "k8s.io/api/core/v1"
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
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	manifestctrltest "github.com/kyma-project/lifecycle-manager/tests/integration/controller/manifest/manifesttest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	testEnv    *envtest.Environment
	k8sManager ctrl.Manager
	reconciler *declarativev2.Reconciler
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
		manifestctrltest.ManifestFilePath = filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples", "oci", "rendered.yaml")
		manifestctrltest.Ctx, manifestctrltest.Cancel = context.WithCancel(context.TODO())
		logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

		// create registry and server
		newReg := registry.New()
		manifestctrltest.Server = httptest.NewServer(newReg)

		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
			ErrorIfCRDPathMissing: false,
		}

		var err error
		cfg, err = testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		//+kubebuilder:scaffold:scheme

		Expect(api.AddToScheme(k8sclientscheme.Scheme)).To(Succeed())
		Expect(apicorev1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

		metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
		if !found {
			metricsBindAddress = ":0"
		}

		k8sManager, err = ctrl.NewManager(
			cfg, ctrl.Options{
				Metrics: metricsserver.Options{
					BindAddress: metricsBindAddress,
				},
				Scheme: k8sclientscheme.Scheme,
				Cache:  internal.GetCacheOptions(k8slabels.Set{v1beta2.ManagedBy: v1beta2.OperatorName}),
			},
		)
		Expect(err).ToNot(HaveOccurred())

		authUser, err := testEnv.AddUser(
			envtest.User{
				Name:   "skr-admin-account",
				Groups: []string{"system:masters"},
			}, cfg,
		)
		Expect(err).NotTo(HaveOccurred())

		manifestctrltest.K8sClient = k8sManager.GetClient()

		kcp := &declarativev2.ClusterInfo{Config: cfg, Client: manifestctrltest.K8sClient}
		reconciler = declarativev2.NewFromManager(
			k8sManager, &v1beta2.Manifest{},
			declarativev2.WithSpecResolver(
				manifest.NewSpecResolver(kcp),
			),
			declarativev2.WithPermanentConsistencyCheck(true),
			declarativev2.WithRemoteTargetCluster(
				func(_ context.Context, _ declarativev2.Object) (*declarativev2.ClusterInfo, error) {
					return &declarativev2.ClusterInfo{Config: authUser.Config()}, nil
				},
			),
			manifest.WithClientCacheKey(),
			declarativev2.WithPostRun{manifest.PostRunCreateCR},
			declarativev2.WithPreDelete{manifest.PreDeleteDeleteCR},
			declarativev2.WithCustomReadyCheck(declarativev2.NewExistsReadyCheck()),
		)

		err = ctrl.NewControllerManagedBy(k8sManager).
			For(&v1beta2.Manifest{}).
			Watches(&apicorev1.Secret{}, handler.Funcs{}).
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
			err = k8sManager.Start(manifestctrltest.Ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}()
	},
)

var _ = AfterSuite(
	func() {
		manifestctrltest.Cancel()
		By("tearing down the test environment")
		manifestctrltest.Server.Close()
		Eventually(func() error { return testEnv.Stop() }, standardTimeout, standardInterval).Should(Succeed())
	},
)
