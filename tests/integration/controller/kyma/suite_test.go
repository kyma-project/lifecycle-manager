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
package kyma_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	testskrcontext "github.com/kyma-project/lifecycle-manager/pkg/testutils/skrcontextimpl"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const randomPort = "0"

var (
	kcpClient          client.Client
	mgr                manager.Manager
	controlPlaneEnv    *envtest.Environment
	ctx                context.Context
	cancel             context.CancelFunc
	cfg                *rest.Config
	descriptorProvider *provider.CachedDescriptorProvider
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
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

	cfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err = ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: randomPort,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache:  internal.GetCacheOptions(false, "istio-system", "kcp-system", "kyma-system"),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
		Warning: 100 * time.Millisecond,
	}

	remoteClientCache := remote.NewClientCache()
	descriptorProvider = provider.NewCachedDescriptorProvider(nil)
	kcpClient = mgr.GetClient()
	testSkrContextFactory := testskrcontext.NewSingleClusterFactory(kcpClient, mgr.GetConfig())
	err = (&kyma.Reconciler{
		Client:              kcpClient,
		EventRecorder:       mgr.GetEventRecorderFor(shared.OperatorName),
		DescriptorProvider:  descriptorProvider,
		SkrContextFactory:   testSkrContextFactory,
		SyncRemoteCrds:      remote.NewSyncCrdsUseCase(kcpClient, testSkrContextFactory, nil),
		RequeueIntervals:    intervals,
		RemoteClientCache:   remoteClientCache,
		InKCPMode:           false,
		RemoteSyncNamespace: flags.DefaultRemoteSyncNamespace,
		Metrics:             metrics.NewKymaMetrics(metrics.NewSharedMetrics()),
	}).SetupWithManager(mgr, ctrlruntime.Options{},
		kyma.ReconcilerSetupSettings{ListenerAddr: randomPort})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
