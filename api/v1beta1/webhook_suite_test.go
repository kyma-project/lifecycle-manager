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
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-component-model/ocm/pkg/contexts/oci"
	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	_ "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"go.uber.org/zap/zapcore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient            client.Client        //nolint:gochecknoglobals
	testEnv              *envtest.Environment //nolint:gochecknoglobals
	webhookServerContext context.Context      //nolint:gochecknoglobals
	webhookServerCancel  context.CancelFunc   //nolint:gochecknoglobals
	cfg                  *rest.Config         //nolint:gochecknoglobals
	scheme               *runtime.Scheme      //nolint:gochecknoglobals
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
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme = runtime.NewScheme()
	ocm.DefaultContext().RepositoryTypes().Register(genericocireg.Type, &genericocireg.RepositoryType{})
	ocm.DefaultContext().RepositoryTypes().Register(genericocireg.TypeV1, &genericocireg.RepositoryType{})
	cpi.DefaultContext().RepositoryTypes().Register(
		ocireg.LegacyType, genericocireg.NewRepositoryType(oci.DefaultContext()),
	)
	Expect(api.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(apiextensionsv1.AddToScheme(scheme)).NotTo(HaveOccurred())
	Expect(admissionv1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

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
	mgr, err := ctrl.NewManager(
		cfg, ctrl.Options{
			Scheme:             scheme,
			Host:               webhookInstallOptions.LocalServingHost,
			Port:               webhookInstallOptions.LocalServingPort,
			CertDir:            webhookInstallOptions.LocalServingCertDir,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
	Expect(err).NotTo(HaveOccurred())

	Expect((&v1beta1.ModuleTemplate{}).SetupWebhookWithManager(mgr)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

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
