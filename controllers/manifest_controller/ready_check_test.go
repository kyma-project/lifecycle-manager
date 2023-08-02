package manifest_controller_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
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
			PushToRemoteOCIRegistry(installName)
		},
	)
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), customDir))).To(Succeed())
		},
	)
	It("Install OCI specs including an nginx deployment", func() {
		testManifest := NewTestManifest("custom-check-oci")
		manifestName := testManifest.GetName()
		validImageSpec := createOCIImageSpec(installName, server.Listener.Addr().String(), false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(installManifest(testManifest, imageSpecByte, false)).To(Succeed())

		Eventually(expectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient()
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment and Sample CR are deployed and ready")
		deploy := &appsv1.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Preparing resources for the CR readiness check")
		resources, err := prepareResourceInfosForCustomCheck(testClient, deploy)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).ToNot(BeEmpty())

		By("Executing the CR readiness check")
		customReadyCheck := manifest.NewCustomResourceReadyCheck()
		stateInfo, err := customReadyCheck.Run(ctx, testClient, testManifest, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(stateInfo.State).To(Equal(declarative.StateReady))

		By("cleaning up the manifest")
		Eventually(deleteManifestAndVerify(testManifest), standardTimeout, standardInterval).Should(Succeed())
	})
})

func verifyDeploymentInstallation(deploy *appsv1.Deployment) error {
	err := k8sClient.Get(
		ctx, client.ObjectKey{
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
	err = k8sClient.Status().Update(ctx, deploy)
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
		Client: k8sClient,
	}

	return declarative.NewSingletonClients(cluster)
}
