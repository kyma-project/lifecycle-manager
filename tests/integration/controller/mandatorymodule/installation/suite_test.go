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

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"

	"go.uber.org/zap/zapcore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sclientscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/controller"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	useRandomPort = "0"
)

var (
	mandatoryModuleReconciler *controller.MandatoryModuleReconciler
	controlPlaneClient        client.Client
	singleClusterEnv          *envtest.Environment
	ctx                       context.Context
	cancel                    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Purge Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))

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

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: useRandomPort,
			},
			Scheme: k8sclientscheme.Scheme,
			Cache:  internal.DefaultCacheOptions(),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
		Warning: 100 * time.Millisecond,
	}

	descriptorProvider := provider.NewCachedDescriptorProvider(nil)
	mandatoryModuleReconciler = &controller.MandatoryModuleReconciler{
		Client:              k8sManager.GetClient(),
		EventRecorder:       k8sManager.GetEventRecorderFor(shared.OperatorName),
		DescriptorProvider:  descriptorProvider,
		RequeueIntervals:    intervals,
		RemoteSyncNamespace: flags.DefaultRemoteSyncNamespace,
		InKCPMode:           false,
	}

	err = mandatoryModuleReconciler.SetupWithManager(k8sManager, ctrlruntime.Options{})
	Expect(err).ToNot(HaveOccurred())

	controlPlaneClient = k8sManager.GetClient()

	SetDefaultEventuallyPollingInterval(Interval)
	SetDefaultEventuallyTimeout(Timeout)
	SetDefaultConsistentlyDuration(ConsistentCheckTimeout)
	SetDefaultConsistentlyPollingInterval(Interval)

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := singleClusterEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
