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
package control_plane_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/kyma-project/lifecycle-manager/internal/controller"

	operatorv1beta2 "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"

	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"go.uber.org/zap/zapcore"

	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	ctrl "sigs.k8s.io/controller-runtime"
	controllerRuntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const UseRandomPort = "0"

var (
	controlPlaneClient client.Client
	k8sManager         manager.Manager
	controlPlaneEnv    *envtest.Environment
	ctx                context.Context
	cancel             context.CancelFunc
	cfg                *rest.Config
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
		filepath.Join("..", "..", "..", "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")
	Expect(err).ToNot(HaveOccurred())

	kcpModuleCRD := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "..", "..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &kcpModuleCRD)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{kcpModuleCRD}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(api.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(
		cfg, ctrl.Options{
			Metrics: metricsserver.Options{
				BindAddress: UseRandomPort,
			},
			Scheme: scheme.Scheme,
			Cache:  controller.NewCacheOptions(),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := queue.RequeueIntervals{
		Success: 1 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
	}

	remoteClientCache := remote.NewClientCache()
	err = (&controller.KymaReconciler{
		Client:           k8sManager.GetClient(),
		EventRecorder:    k8sManager.GetEventRecorderFor(operatorv1beta2.OperatorName),
		RequeueIntervals: intervals,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache:   remoteClientCache,
		KcpRestConfig:       k8sManager.GetConfig(),
		InKCPMode:           true,
		RemoteSyncNamespace: controller.DefaultRemoteSyncNamespace,
		IsManagedKyma:       true,
	}).SetupWithManager(k8sManager, controllerRuntime.Options{},
		controller.SetupUpSetting{ListenerAddr: UseRandomPort})
	Expect(err).ToNot(HaveOccurred())

	controlPlaneClient = k8sManager.GetClient()

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
