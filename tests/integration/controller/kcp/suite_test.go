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
package kcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"go.uber.org/zap/zapcore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/crd"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator/fromerror"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	testskrcontext "github.com/kyma-project/lifecycle-manager/tests/integration/commontestutils/skrcontextimpl"

	_ "ocm.software/ocm/api/ocm"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const UseRandomPort = "0"

var (
	mgr                   manager.Manager
	kcpClient             client.Client
	testSkrContextFactory *testskrcontext.DualClusterFactory
	kcpEnv                *envtest.Environment
	ctx                   context.Context
	cancel                context.CancelFunc
	cfg                   *rest.Config
	descriptorProvider    *provider.CachedDescriptorProvider
	crdCache              *crd.Cache
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logr := log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter))
	logf.SetLogger(logr)
	var err error
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

	kcpEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		CRDs:                  append([]*apiextensionsv1.CustomResourceDefinition{kcpModuleCRD}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = kcpEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err = ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: UseRandomPort,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache: setup.SetupCacheOptions(false,
				"istio-system",
				ControlPlaneNamespace,
				certmanagerv1.SchemeGroupVersion.String(),
				logr),
		})
	Expect(err).ToNot(HaveOccurred())

	kcpClient = mgr.GetClient()

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
		Warning: 100 * time.Millisecond,
	}

	Expect(err).NotTo(HaveOccurred())

	testEventRec := event.NewRecorderWrapper(mgr.GetEventRecorderFor(shared.OperatorName))
	testSkrContextFactory = testskrcontext.NewDualClusterFactory(kcpClient.Scheme(), testEventRec)
	descriptorProvider = provider.NewCachedDescriptorProvider()
	crdCache = crd.NewCache(nil)
	noOpMetricsFunc := func(kymaName, moduleName string) {}
	moduleStatusGen := generator.NewModuleStatusGenerator(fromerror.GenerateModuleStatusFromError)
	err = (&kyma.Reconciler{
		Client:               kcpClient,
		SkrContextFactory:    testSkrContextFactory,
		Event:                testEventRec,
		RequeueIntervals:     intervals,
		DescriptorProvider:   descriptorProvider,
		SyncRemoteCrds:       remote.NewSyncCrdsUseCase(kcpClient, testSkrContextFactory, crdCache),
		ModulesStatusHandler: modules.NewStatusHandler(moduleStatusGen, kcpClient, noOpMetricsFunc),
		RemoteSyncNamespace:  flags.DefaultRemoteSyncNamespace,
		IsManagedKyma:        true,
		Metrics:              metrics.NewKymaMetrics(metrics.NewSharedMetrics()),
		RemoteCatalog: remote.NewRemoteCatalogFromKyma(kcpClient, testSkrContextFactory,
			flags.DefaultRemoteSyncNamespace),
		TemplateLookup: templatelookup.NewTemplateLookup(kcpClient, descriptorProvider,
			moduletemplateinfolookup.NewModuleTemplateInfoLookupStrategies(
				[]moduletemplateinfolookup.ModuleTemplateInfoLookupStrategy{
					moduletemplateinfolookup.NewByVersionStrategy(kcpClient),
					moduletemplateinfolookup.NewByChannelStrategy(kcpClient),
					moduletemplateinfolookup.NewByModuleReleaseMetaStrategy(kcpClient),
				})),
	}).SetupWithManager(mgr, ctrlruntime.Options{},
		kyma.SetupOptions{ListenerAddr: UseRandomPort})
	Expect(err).ToNot(HaveOccurred())
	Eventually(CreateNamespace, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, ControlPlaneNamespace).Should(Succeed())
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	Expect(kcpEnv.Stop()).To(Succeed())
	Expect(testSkrContextFactory.Stop()).To(Succeed())
})
