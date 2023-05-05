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

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	"testing"

	//nolint:gci
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	kcpConfigEnvVar = "KCP_KUBECONFIG"
	skrConfigEnvVar = "SKR_KUBECONFIG"
)

var (
	controlPlaneEnv        *envtest.Environment //nolint:gochecknoglobals
	controlPlaneClient     client.Client        //nolint:gochecknoglobals
	controlPlaneRESTConfig *rest.Config         //nolint:gochecknoglobals

	runtimeEnv        *envtest.Environment //nolint:gochecknoglobals
	runtimeClient     client.Client        //nolint:gochecknoglobals
	runtimeRESTConfig *rest.Config         //nolint:gochecknoglobals

	ctx    context.Context    //nolint:gochecknoglobals
	cancel context.CancelFunc //nolint:gochecknoglobals

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

	// k8s configs
	controlPlaneConfig, runtimeConfig, err := getConfigs()
	Expect(err).ToNot(HaveOccurred())
	existingCluster := true
	controlPlaneRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(controlPlaneConfig)
	runtimeRESTConfig, err = clientcmd.RESTConfigFromKubeConfig(runtimeConfig)
	Expect(err).NotTo(HaveOccurred())

	controlPlaneEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("../..", "config", "crd", "bases")},
		CRDs:                  append([]*v1.CustomResourceDefinition{controlplaneCrd}, externalCRDs...),
		ErrorIfCRDPathMissing: true,
		UseExistingCluster:    &existingCluster,
		Config:                controlPlaneRESTConfig,
	}
	controlPlaneClient, err = client.New(controlPlaneRESTConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	runtimeClient, err = client.New(runtimeRESTConfig, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	_, err = controlPlaneEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(controlPlaneEnv.Config).NotTo(BeNil())

	Expect(api.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())
	Expect(v1.AddToScheme(scheme.Scheme)).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	go func() {
		defer GinkgoRecover()
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()

	err := controlPlaneEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func getConfigs() ([]byte, []byte, error) {
	controlplaneConfigFile := os.Getenv(kcpConfigEnvVar)
	if controlplaneConfigFile == "" {
		return nil, nil, errors.New(fmt.Sprintf("'%s' is empty", kcpConfigEnvVar))
	}
	controlplaneConfig, err := os.ReadFile(controlplaneConfigFile)
	if err != nil {
		return nil, nil, err
	}

	runtimeConfigFile := os.Getenv(skrConfigEnvVar)
	if runtimeConfigFile == "" {
		return nil, nil, errors.New(fmt.Sprintf("'%s' is empty", skrConfigEnvVar))
	}
	runtimeConfig, err := os.ReadFile(runtimeConfigFile)
	if err != nil {
		return nil, nil, err
	}

	return controlplaneConfig, runtimeConfig, nil
}
