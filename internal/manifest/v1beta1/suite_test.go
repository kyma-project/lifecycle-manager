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

package v1beta1_test

import (
	"context"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/internal"
	internalv1beta1 "github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	//+kubebuilder:scaffold:imports

	"github.com/kyma-project/lifecycle-manager/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient     client.Client                                       //nolint:gochecknoglobals
	testEnv       *envtest.Environment                                //nolint:gochecknoglobals
	k8sManager    ctrl.Manager                                        //nolint:gochecknoglobals
	ctx           context.Context                                     //nolint:gochecknoglobals
	cancel        context.CancelFunc                                  //nolint:gochecknoglobals
	server        *httptest.Server                                    //nolint:gochecknoglobals
	helmCacheRepo = filepath.Join(helmCacheHome, "repository")        //nolint:gochecknoglobals
	helmRepoFile  = filepath.Join(helmCacheHome, "repositories.yaml") //nolint:gochecknoglobals
	reconciler    *declarative.Reconciler                             //nolint:gochecknoglobals
	cfg           *rest.Config                                        //nolint:gochecknoglobals
)

const (
	helmCacheHomeEnv   = "HELM_CACHE_HOME"
	helmCacheHome      = "/tmp/caches"
	helmCacheRepoEnv   = "HELM_REPOSITORY_CACHE"
	helmRepoEnv        = "HELM_REPOSITORY_CONFIG"
	kustomizeLocalPath = "../../../pkg/test_samples/kustomize"
	standardTimeout    = 30 * time.Second
	standardInterval   = 100 * time.Millisecond
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(
	func() {
		err := os.RemoveAll(helmCacheHome)
		Expect(err != nil && !os.IsExist(err)).To(BeFalse())
		Expect(os.MkdirAll(helmCacheHome, os.ModePerm)).NotTo(HaveOccurred())

		ctx, cancel = context.WithCancel(context.TODO())
		logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

		// create registry and server
		newReg := registry.New()
		server = httptest.NewServer(newReg)

		By("bootstrapping test environment")
		testEnv = &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: false,
		}

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

		k8sManager, err = ctrl.NewManager(
			cfg, ctrl.Options{
				MetricsBindAddress: metricsBindAddress,
				Scheme:             scheme.Scheme,
				NewCache:           internal.GetCacheFunc(labels.Set{v1beta1.ManagedBy: v1beta1.OperatorName}),
			},
		)
		Expect(err).ToNot(HaveOccurred())
		codec, err := v1beta1.NewCodec()
		Expect(err).ToNot(HaveOccurred())

		var authUser *envtest.AuthenticatedUser
		authUser, err = testEnv.AddUser(
			envtest.User{
				Name:   "skr-admin-account",
				Groups: []string{"system:masters"},
			}, cfg,
		)
		Expect(err).NotTo(HaveOccurred())

		k8sClient = k8sManager.GetClient()

		kcp := &declarative.ClusterInfo{Config: cfg, Client: k8sClient}
		reconciler = declarative.NewFromManager(
			k8sManager, &v1beta1.Manifest{},
			declarative.WithSpecResolver(
				internalv1beta1.NewManifestSpecResolver(kcp, codec),
			),
			declarative.WithPermanentConsistencyCheck(true),
			declarative.WithRemoteTargetCluster(
				func(_ context.Context, _ declarative.Object) (*declarative.ClusterInfo, error) {
					return &declarative.ClusterInfo{Config: authUser.Config()}, nil
				},
			),
			internalv1beta1.WithClientCacheKey(),
			declarative.WithPostRun{internalv1beta1.PostRunCreateCR},
			declarative.WithPreDelete{internalv1beta1.PreDeleteDeleteCR},
			declarative.WithCustomReadyCheck(declarative.NewExistsReadyCheck()),
		)

		err = ctrl.NewControllerManagedBy(k8sManager).
			For(&v1beta1.Manifest{}).
			Watches(&source.Kind{Type: &v1.Secret{}}, handler.Funcs{}).
			WithOptions(
				controller.Options{
					RateLimiter: internal.ManifestRateLimiter(
						1*time.Second, 1000*time.Second,
						30, 200,
					),
					MaxConcurrentReconciles: 1,
				},
			).Complete(reconciler)
		Expect(err).ToNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			err = k8sManager.Start(ctx)
			Expect(err).ToNot(HaveOccurred(), "failed to run manager")
		}()
	},
)

var _ = AfterSuite(
	func() {
		cancel()
		By("tearing down the test environment")
		server.Close()
		Eventually(func() error { return testEnv.Stop() }, standardTimeout, standardInterval).Should(Succeed())
		err := os.RemoveAll(helmCacheHome)
		Expect(err).NotTo(HaveOccurred())
	},
)
