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
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	kymacontroller "github.com/kyma-project/kyma-operator/operator/controllers"
	"github.com/kyma-project/kyma-operator/operator/pkg/signature"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const listenerAddr = ":8082"

var (
	_          *rest.Config
	k8sClient  client.Client        //nolint:gochecknoglobals
	k8sManager manager.Manager      //nolint:gochecknoglobals
	testEnv    *envtest.Environment //nolint:gochecknoglobals
	ctx        context.Context      //nolint:gochecknoglobals
	cancel     context.CancelFunc   //nolint:gochecknoglobals
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

	manifestCrd := &v1.CustomResourceDefinition{}
	res, err := http.DefaultClient.Get(
		"https://raw.githubusercontent.com/kyma-project/manifest-operator/main/operator/config/crd/bases/component.kyma-project.io_manifests.yaml") //nolint:lll
	Expect(err).NotTo(HaveOccurred())
	Expect(res.StatusCode).To(BeEquivalentTo(http.StatusOK))
	Expect(yaml2.NewYAMLOrJSONDecoder(res.Body, 2048).Decode(manifestCrd)).To(Succeed())

	controlplaneCrd := &v1.CustomResourceDefinition{}
	modulePath := filepath.Join("..", "config", "samples", "component-integration-installed",
		"crd", "component.kyma-project.io_controlplanemodules.yaml")
	moduleFile, err := os.ReadFile(modulePath)
	Expect(err).To(BeNil())
	Expect(moduleFile).ToNot(BeEmpty())
	Expect(yaml2.Unmarshal(moduleFile, &controlplaneCrd)).To(Succeed())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		CRDs:                  []*v1.CustomResourceDefinition{manifestCrd, controlplaneCrd},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	Expect(operatorv1alpha1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:   scheme.Scheme,
		NewCache: kymacontroller.NewCacheFunc(),
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&kymacontroller.KymaReconciler{
		Client:        k8sManager.GetClient(),
		EventRecorder: k8sManager.GetEventRecorderFor(operatorv1alpha1.OperatorName),
		RequeueIntervals: kymacontroller.RequeueIntervals{
			Success: 3 * time.Second,
			Failure: 1 * time.Second,
			Waiting: 1 * time.Second,
		},
		VerificationSettings: signature.VerificationSettings{
			EnableVerification: false,
		},
	}).SetupWithManager(k8sManager, controller.Options{}, listenerAddr)
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
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
