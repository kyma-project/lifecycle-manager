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

package control_plane_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	operatorv1beta2 "github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/open-component-model/ocm/pkg/contexts/oci"
	"github.com/open-component-model/ocm/pkg/contexts/oci/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"go.uber.org/zap/zapcore"

	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/controllers"
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
	controlPlaneClient client.Client        //nolint:gochecknoglobals
	k8sManager         manager.Manager      //nolint:gochecknoglobals
	controlPlaneEnv    *envtest.Environment //nolint:gochecknoglobals
	ctx                context.Context      //nolint:gochecknoglobals
	cancel             context.CancelFunc   //nolint:gochecknoglobals
	cfg                *rest.Config         //nolint:gochecknoglobals
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
		filepath.Join("../..", "config", "samples", "tests", "crds"),
		"cert-manager-v1.10.1.crds.yaml",
		"istio-v1.17.1.crds.yaml")

	// kcpModule CRD
	controlplaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("../..", "config", "samples", "component-integration-installed",
		"crd", "operator.kyma-project.io_kcpmodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).ToNot(HaveOccurred())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &controlplaneCrd)).To(Succeed())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlplaneCrd}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	ocm.DefaultContext().RepositoryTypes().Register(genericocireg.Type, &genericocireg.RepositoryType{})
	ocm.DefaultContext().RepositoryTypes().Register(genericocireg.TypeV1, &genericocireg.RepositoryType{})
	cpi.DefaultContext().RepositoryTypes().Register(
		ocireg.LegacyType, genericocireg.NewRepositoryType(oci.DefaultContext()),
	)
	Expect(api.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(
		cfg, ctrl.Options{
			MetricsBindAddress: UseRandomPort,
			Scheme:             scheme.Scheme,
			NewCache:           controllers.NewCacheFunc(),
		})
	Expect(err).ToNot(HaveOccurred())

	intervals := controllers.RequeueIntervals{
		Success: 3 * time.Second,
	}

	remoteClientCache := remote.NewClientCache()
	err = (&controllers.KymaReconciler{
		Client:           k8sManager.GetClient(),
		EventRecorder:    k8sManager.GetEventRecorderFor(operatorv1beta2.OperatorName),
		RequeueIntervals: intervals,
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
		RemoteClientCache:   remoteClientCache,
		KcpRestConfig:       k8sManager.GetConfig(),
		InKCPMode:           true,
		RemoteSyncNamespace: controllers.DefaultRemoteSyncNamespace,
		IsManagedKyma:       true,
	}).SetupWithManager(k8sManager, controller.Options{},
		controllers.SetupUpSetting{ListenerAddr: UseRandomPort})
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
