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

package controllers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	moduleManagerV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/signature"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const listenerAddr = ":8082"

var (
	controlPlaneClient client.Client        //nolint:gochecknoglobals
	runtimeClient      client.Client        //nolint:gochecknoglobals
	k8sManager         manager.Manager      //nolint:gochecknoglobals
	controlPlaneEnv    *envtest.Environment //nolint:gochecknoglobals
	runtimeEnv         *envtest.Environment //nolint:gochecknoglobals
	ctx                context.Context      //nolint:gochecknoglobals
	cancel             context.CancelFunc   //nolint:gochecknoglobals
	cfg                *rest.Config         //nolint:gochecknoglobals
)

const (
	webhookChartPath       = "../internal/charts/skr-webhook"
	istioResourcesFilePath = "../internal/assets/istio-test-resources.yaml"
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping test environment")

	// manifest CRD
	// istio CRDs
	remoteCrds, err := parseRemoteCRDs([]string{
		"https://raw.githubusercontent.com/kyma-project/module-manager/main/operator/config/crd/bases/operator.kyma-project.io_manifests.yaml", //nolint:lll
		"https://raw.githubusercontent.com/istio/istio/master/manifests/charts/base/crds/crd-all.gen.yaml",                                     //nolint:lll
	})
	Expect(err).NotTo(HaveOccurred())

	// kcpModule CRD
	controlplaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).To(BeNil())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &controlplaneCrd)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlplaneCrd}, remoteCrds...),
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(operatorv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(moduleManagerV1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	controlPlaneClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(controlPlaneClient).NotTo(BeNil())

	runtimeClient, runtimeEnv = NewSKRCluster()

	metricsBindAddress, found := os.LookupEnv("metrics-bind-address")
	if !found {
		metricsBindAddress = ":8080"
	}

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		MetricsBindAddress: metricsBindAddress,
		Scheme:             scheme.Scheme,
		NewCache:           controllers.NewCacheFunc(),
	})
	Expect(err).ToNot(HaveOccurred())

	intervals := controllers.RequeueIntervals{
		Success: 3 * time.Second,
		Failure: 1 * time.Second,
		Waiting: 1 * time.Second,
	}

	remoteClientCache := remote.NewClientCache()

	err = (&controllers.KymaReconciler{
		Client:           k8sManager.GetClient(),
		EventRecorder:    k8sManager.GetEventRecorderFor(operatorv1alpha1.OperatorName),
		RequeueIntervals: intervals,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache: remoteClientCache,
	}).SetupWithManager(k8sManager, controller.Options{}, listenerAddr)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.WatcherReconciler{
		Client:           k8sManager.GetClient(),
		RestConfig:       k8sManager.GetConfig(),
		Scheme:           scheme.Scheme,
		RequeueIntervals: intervals,
		Config: &controllers.WatcherConfig{
			WebhookChartPath:       webhookChartPath,
			SkrWebhookMemoryLimits: "200Mi",
			SkrWebhookCPULimits:    "1",
		},
	}).SetupWithManager(k8sManager, controller.Options{})
	Expect(err).ToNot(HaveOccurred())

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
	err = runtimeEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func NewSKRCluster() (client.Client, *envtest.Environment) {
	skrEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := skrEnv.Start()
	Expect(cfg).NotTo(BeNil())
	Expect(err).NotTo(HaveOccurred())

	var authUser *envtest.AuthenticatedUser
	authUser, err = skrEnv.AddUser(envtest.User{
		Name:   "skr-admin-account",
		Groups: []string{"system:masters"},
	}, cfg)
	Expect(err).NotTo(HaveOccurred())

	remote.LocalClient = func() *rest.Config {
		return authUser.Config()
	}

	skrClient, err := client.New(authUser.Config(), client.Options{Scheme: controlPlaneClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())

	return skrClient, skrEnv
}
