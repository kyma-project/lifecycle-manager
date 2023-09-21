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

	hlp "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerRuntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
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
		hlp.ManifestFilePath = "../../../pkg/test_samples/oci/rendered.yaml"
		hlp.Ctx, hlp.Cancel = context.WithCancel(context.TODO())
		logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

		// create registry and server
		newReg := registry.New()
		hlp.Server = httptest.NewServer(newReg)

		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: false,
		}

		var err error
		cfg, err = testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		//+kubebuilder:scaffold:scheme

		Expect(api.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

		metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
		if !found {
			metricsBindAddress = ":0"
		}

		syncPeriod := 2 * time.Second

		k8sManager, err = ctrl.NewManager(
			cfg, ctrl.Options{
				MetricsBindAddress: metricsBindAddress,
				Scheme:             scheme.Scheme,
				Cache:              internal.GetCacheOptions(labels.Set{v1beta2.ManagedBy: v1beta2.OperatorName}),
				SyncPeriod:         &syncPeriod,
			},
		)

		k8sManager.GetControllerOptions()
		Expect(err).ToNot(HaveOccurred())
		codec, err := v1beta2.NewCodec()
		Expect(err).ToNot(HaveOccurred())

		authUser, err := testEnv.AddUser(
			envtest.User{
				Name:   "skr-admin-account",
				Groups: []string{"system:masters"},
			}, cfg,
		)
		Expect(err).NotTo(HaveOccurred())

		hlp.K8sClient = k8sManager.GetClient()

		kcp := &declarative.ClusterInfo{Config: cfg, Client: hlp.K8sClient}
		reconciler = declarative.NewFromManager(
			k8sManager, &v1beta2.Manifest{},
			declarative.WithSpecResolver(
				manifest.NewSpecResolver(kcp, codec),
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
			declarative.WithModuleCRDName(manifest.GetModuleCRDName),
		)

		err = ctrl.NewControllerManagedBy(k8sManager).
			For(&v1beta2.Manifest{}).
			Watches(&v1.Secret{}, handler.Funcs{}).
			WithOptions(
				controllerRuntime.Options{
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
			err = k8sManager.Start(hlp.Ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}()
	},
)

var _ = AfterSuite(
	func() {
		hlp.Cancel()
		By("tearing down the test environment")
		hlp.Server.Close()
		Eventually(func() error { return testEnv.Stop() }, standardTimeout, standardInterval).Should(Succeed())
	},
)
