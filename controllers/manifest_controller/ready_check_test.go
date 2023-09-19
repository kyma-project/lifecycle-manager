package manifest_controller_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	hlp "github.com/kyma-project/lifecycle-manager/controllers/manifest_controller/manifesttest"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Manifest readiness check", Ordered, func() {
	customDir := "custom-dir"
	installName := filepath.Join(customDir, "installs")
	It(
		"setup OCI", func() {
			hlp.PushToRemoteOCIRegistry(installName)
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
		validImageSpec := hlp.CreateOCIImageSpec(installName, hlp.Server.Listener.Addr().String(), false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(hlp.InstallManifest(testManifest, imageSpecByte, false)).To(Succeed())

		Eventually(hlp.ExpectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient()
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment is deployed and ready")
		deploy := &appsv1.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Verifying manifest status contains all resources")
		status, err := hlp.GetManifestStatus(manifestName)
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
		stateInfo, err := customReadyCheck.Run(hlp.Ctx, testClient, testManifest, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(stateInfo.State).To(Equal(declarative.StateReady))

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).Should(BeTrue())

		Eventually(hlp.DeleteManifestAndVerify(testManifest), standardTimeout, standardInterval).Should(Succeed())

		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
	})
})

func verifyDeploymentInstallation(deploy *appsv1.Deployment) error {
	err := hlp.K8sClient.Get(
		hlp.Ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
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
		appsv1.DeploymentCondition{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		})
	err = hlp.K8sClient.Status().Update(hlp.Ctx, deploy)
	if err != nil {
		return err
	}
	return nil
}

func prepareResourceInfosForCustomCheck(clt declarative.Client, deploy *appsv1.Deployment) ([]*resource.Info, error) {
	deployUnstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return nil, err
	}
	deployUnstructured := &unstructured.Unstructured{}
	deployUnstructured.SetUnstructuredContent(deployUnstructuredObj)
	deployUnstructured.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	deployInfo, err := clt.ResourceInfo(deployUnstructured, true)
	if err != nil {
		return nil, err
	}
	return []*resource.Info{deployInfo}, nil
}

func declarativeTestClient() (declarative.Client, error) {
	cluster := &declarative.ClusterInfo{
		Config: cfg,
		Client: hlp.K8sClient,
	}

	return declarative.NewSingletonClients(cluster)
}

func asResource(name, namespace, group, version, kind string) declarative.Resource {
	return declarative.Resource{
		Name: name, Namespace: namespace,
		GroupVersionKind: metav1.GroupVersionKind{
			Group: group, Version: version, Kind: kind,
		},
	}
}

func verifyObjectExists(obj *unstructured.Unstructured) func() (bool, error) {
	return func() (bool, error) {
		err := hlp.K8sClient.Get(
			hlp.Ctx, client.ObjectKeyFromObject(obj),
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
