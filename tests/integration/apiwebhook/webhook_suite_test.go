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
package apiwebhook_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	apiadmissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	pkgApi "github.com/kyma-project/lifecycle-manager/pkg/api"
	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	_ "github.com/open-component-model/ocm/pkg/contexts/ocm"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient            client.Client
	testEnv              *envtest.Environment
	webhookServerContext context.Context
	webhookServerCancel  context.CancelFunc
	cfg                  *rest.Config
	scheme               *machineryruntime.Scheme
)

const (
	Timeout  = time.Second * 10
	Interval = time.Millisecond * 250
)

func TestAPIs(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(log.ConfigLogger(9, zapcore.AddSync(GinkgoWriter)))
	webhookServerContext, webhookServerCancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(integration.GetProjectRoot(), "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join(integration.GetProjectRoot(), "config", "webhook")},
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme = machineryruntime.NewScheme()
	Expect(pkgApi.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(apiadmissionv1.AddToScheme(scheme)).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	SetupWebhook()
})

func StopWebhook() {
	webhookServerCancel()
	<-webhookServerContext.Done()
}

func SetupWebhook() {
	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	webhookOpts := webhook.Options{
		Host:    webhookInstallOptions.LocalServingHost,
		Port:    webhookInstallOptions.LocalServingPort,
		CertDir: webhookInstallOptions.LocalServingCertDir,
	}
	mgr, err := ctrl.NewManager(
		cfg, ctrl.Options{
			Scheme:         scheme,
			WebhookServer:  webhook.NewServer(webhookOpts),
			LeaderElection: false,
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
		})
	Expect(err).NotTo(HaveOccurred())

	Expect((&pkgapiv1beta2.ModuleTemplateInCtrlRuntime{}).SetupWebhookWithManager(mgr)).NotTo(HaveOccurred())
	Expect((&pkgapiv1beta2.KymaInCtrlRuntime{}).SetupWebhookWithManager(mgr)).NotTo(HaveOccurred())
	Expect((&pkgapiv1beta2.ManifestInCtrlRuntime{}).SetupWebhookWithManager(mgr)).NotTo(HaveOccurred())
	Expect((&pkgapiv1beta2.WatcherInCtrlRuntime{}).SetupWebhookWithManager(mgr)).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(webhookServerContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		})
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}, Timeout, Interval).Should(Succeed())
}

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	StopWebhook()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
