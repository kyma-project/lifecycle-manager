package manifest_controller_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	apiapps "k8s.io/api/apps/v1"
	apicore "k8s.io/api/core/v1"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"
	hlp "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest readiness check", Ordered, func() {
	customDir := "custom-dir"
	installName := filepath.Join(customDir, "installs")
	It(
		"setup OCI", func() {
			manifesttest.PushToRemoteOCIRegistry(installName)
		},
	)
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), customDir))).To(Succeed())
		},
	)
	It("Install OCI specs including an nginx deployment", func() {
		testManifest := testutils.NewTestManifest("custom-check-oci")
		manifestName := testManifest.GetName()
		validImageSpec := manifesttest.CreateOCIImageSpec(installName, manifesttest.Server.Listener.Addr().String(), false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(manifesttest.InstallManifest(testManifest, imageSpecByte, false)).To(Succeed())

		Eventually(hlp.ExpectManifestStateIn(shared.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient()
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment is deployed and ready")
		deploy := &apiapps.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Verifying manifest status contains all resources")
		status, err := manifesttest.GetManifestStatus(manifestName)
		Expect(err).ToNot(HaveOccurred())
		Expect(status.Synced).To(HaveLen(2))

		expectedDeployment := asResource("nginx-deployment", "default", "apps", "v1", "Deployment")
		expectedCRD := asResource("samples.operator.kyma-project.io", "",
			"apiextensions.k8s.io", "v1", "CustomResourceDefinition")
		Expect(status.Synced).To(ContainElement(expectedDeployment))
		Expect(status.Synced).To(ContainElement(expectedCRD))

		By("Preparing resources for the CR readiness check")
		resources, err := prepareResourceInfosForCustomCheck(testClient, deploy)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).ToNot(BeEmpty())

		By("Executing the CR readiness check")
		customReadyCheck := manifest.NewCustomResourceReadyCheck()
		stateInfo, err := customReadyCheck.Run(manifesttest.Ctx, testClient, testManifest, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(stateInfo.State).To(Equal(shared.StateReady))

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).Should(BeTrue())

		Eventually(manifesttest.DeleteManifestAndVerify(testManifest), standardTimeout, standardInterval).Should(Succeed())

		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
	})
})

func verifyDeploymentInstallation(deploy *apiapps.Deployment) error {
	err := manifesttest.K8sClient.Get(
		manifesttest.Ctx, client.ObjectKey{
			Namespace: apimachinerymeta.NamespaceDefault,
			Name:      "nginx-deployment",
		}, deploy,
	)
	if err != nil {
		return err
	}
	deploy.Status.Replicas = *deploy.Spec.Replicas
	deploy.Status.ReadyReplicas = *deploy.Spec.Replicas
	deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
	deploy.Status.Conditions = append(deploy.Status.Conditions,
		apiapps.DeploymentCondition{
			Type:   apiapps.DeploymentAvailable,
			Status: apicore.ConditionTrue,
		})
	err = manifesttest.K8sClient.Status().Update(manifesttest.Ctx, deploy)
	if err != nil {
		return err
	}
	return nil
}

func prepareResourceInfosForCustomCheck(clt declarative.Client, deploy *apiapps.Deployment) ([]*resource.Info, error) {
	deployUnstructuredObj, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return nil, err
	}
	deployUnstructured := &unstructured.Unstructured{}
	deployUnstructured.SetUnstructuredContent(deployUnstructuredObj)
	deployUnstructured.SetGroupVersionKind(apiapps.SchemeGroupVersion.WithKind("Deployment"))
	deployInfo, err := clt.ResourceInfo(deployUnstructured, true)
	if err != nil {
		return nil, err
	}
	return []*resource.Info{deployInfo}, nil
}

func declarativeTestClient() (declarative.Client, error) {
	cluster := &declarative.ClusterInfo{
		Config: cfg,
		Client: manifesttest.K8sClient,
	}

	return declarative.NewSingletonClients(cluster)
}

func asResource(name, namespace, group, version, kind string) shared.Resource {
	return shared.Resource{
		Name: name, Namespace: namespace,
		GroupVersionKind: apimachinerymeta.GroupVersionKind{
			Group: group, Version: version, Kind: kind,
		},
	}
}

func verifyObjectExists(obj *unstructured.Unstructured) func() (bool, error) {
	return func() (bool, error) {
		err := manifesttest.K8sClient.Get(
			manifesttest.Ctx, client.ObjectKeyFromObject(obj),
			obj,
		)

		if err == nil {
			return true, nil
		} else if util.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}
}
