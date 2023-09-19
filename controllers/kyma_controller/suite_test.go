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
package kyma_controller_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const randomPort = "0"

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

	externalCRDs := AppendExternalCRDs(
		filepath.Join("..", "..", "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")

	kcpModuleCRD := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &kcpModuleCRD)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
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
			MetricsBindAddress: randomPort,
			Scheme:             scheme.Scheme,
			Cache:              controllers.NewCacheOptions(),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := controllers.RequeueIntervals{
		Success: 3 * time.Second,
		Busy:    100 * time.Millisecond,
		Error:   100 * time.Millisecond,
	}

	remoteClientCache := remote.NewClientCache()

	err = (&controllers.KymaReconciler{
		Client:           k8sManager.GetClient(),
		EventRecorder:    k8sManager.GetEventRecorderFor(v1beta2.OperatorName),
		RequeueIntervals: intervals,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache:   remoteClientCache,
		KcpRestConfig:       k8sManager.GetConfig(),
		InKCPMode:           false,
		RemoteSyncNamespace: controllers.DefaultRemoteSyncNamespace,
	}).SetupWithManager(k8sManager, controller.Options{},
		controllers.SetupUpSetting{ListenerAddr: randomPort})
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
