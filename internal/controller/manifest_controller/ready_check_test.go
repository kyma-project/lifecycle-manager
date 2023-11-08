package manifest_controller_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	manifestctrltest "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
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
			manifestctrltest.PushToRemoteOCIRegistry(installName)
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
		validImageSpec := manifestctrltest.CreateOCIImageSpec(installName, manifestctrltest.Server.Listener.Addr().String(), false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(manifestctrltest.InstallManifest(testManifest, imageSpecByte, false)).To(Succeed())

		Eventually(manifestctrltest.ExpectManifestStateIn(shared.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient()
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment is deployed and ready")
		deploy := &apiappsv1.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Verifying manifest status contains all resources")
		status, err := manifestctrltest.GetManifestStatus(manifestName)
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
		stateInfo, err := customReadyCheck.Run(manifestctrltest.Ctx, testClient, testManifest, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(stateInfo.State).To(Equal(shared.StateReady))

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).Should(BeTrue())

		Eventually(manifestctrltest.DeleteManifestAndVerify(testManifest), standardTimeout, standardInterval).Should(Succeed())

		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
	})
})

func verifyDeploymentInstallation(deploy *apiappsv1.Deployment) error {
	err := manifestctrltest.K8sClient.Get(
		manifestctrltest.Ctx, client.ObjectKey{
			Namespace: apimetav1.NamespaceDefault,
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
		apiappsv1.DeploymentCondition{
			Type:   apiappsv1.DeploymentAvailable,
			Status: apicorev1.ConditionTrue,
		})
	err = manifestctrltest.K8sClient.Status().Update(manifestctrltest.Ctx, deploy)
	if err != nil {
		return err
	}
	return nil
}

func prepareResourceInfosForCustomCheck(clt declarativev2.Client, deploy *apiappsv1.Deployment) ([]*resource.Info, error) {
	deployUnstructuredObj, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return nil, err
	}
	deployUnstructured := &unstructured.Unstructured{}
	deployUnstructured.SetUnstructuredContent(deployUnstructuredObj)
	deployUnstructured.SetGroupVersionKind(apiappsv1.SchemeGroupVersion.WithKind("Deployment"))
	deployInfo, err := clt.ResourceInfo(deployUnstructured, true)
	if err != nil {
		return nil, err
	}
	return []*resource.Info{deployInfo}, nil
}

func declarativeTestClient() (declarativev2.Client, error) {
	cluster := &declarativev2.ClusterInfo{
		Config: cfg,
		Client: manifestctrltest.K8sClient,
	}

	return declarativev2.NewSingletonClients(cluster)
}

func asResource(name, namespace, group, version, kind string) shared.Resource {
	return shared.Resource{
		Name: name, Namespace: namespace,
		GroupVersionKind: apimetav1.GroupVersionKind{
			Group: group, Version: version, Kind: kind,
		},
	}
}

func verifyObjectExists(obj *unstructured.Unstructured) func() (bool, error) {
	return func() (bool, error) {
		err := manifestctrltest.K8sClient.Get(
			manifestctrltest.Ctx, client.ObjectKeyFromObject(obj),
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
