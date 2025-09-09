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

package mandatory_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"go.uber.org/zap/zapcore"
	apicorev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/internal/controller/mandatorymodule"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/setup"
	"github.com/kyma-project/lifecycle-manager/pkg/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	useRandomPort = "0"
)

var (
	reconciler       *mandatorymodule.InstallationReconciler
	kcpClient        client.Client
	singleClusterEnv *envtest.Environment
	ctx              context.Context
	cancel           context.CancelFunc
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Purge Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logr := log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter))
	logf.SetLogger(logr)

	By("bootstrapping test environment")
	singleClusterEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		CRDs:                  []*apiextensionsv1.CustomResourceDefinition{},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := singleClusterEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(api.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())
	Expect(apicorev1.AddToScheme(k8sclientscheme.Scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err := ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: useRandomPort,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache: setup.SetupCacheOptions(false,
				"istio-system",
				ControlPlaneNamespace,
				certmanagerv1.SchemeGroupVersion.String(),
				logr),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
		Warning: 100 * time.Millisecond,
	}

	descriptorProvider := provider.NewCachedDescriptorProvider()
	reconciler = &mandatorymodule.InstallationReconciler{
		Client:              mgr.GetClient(),
		DescriptorProvider:  descriptorProvider,
		RequeueIntervals:    intervals,
		RemoteSyncNamespace: flags.DefaultRemoteSyncNamespace,
		Metrics:             metrics.NewMandatoryModulesMetrics(),
	}

	err = reconciler.SetupWithManager(mgr, ctrlruntime.Options{})
	Expect(err).ToNot(HaveOccurred())

	kcpClient = mgr.GetClient()

	SetDefaultEventuallyPollingInterval(Interval)
	SetDefaultEventuallyTimeout(Timeout)
	SetDefaultConsistentlyDuration(ConsistentCheckTimeout)
	SetDefaultConsistentlyPollingInterval(Interval)
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
	err := singleClusterEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
