package custom_resource_check_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	hlp "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Warning state propagation test", Ordered, func() {
	customDir := "custom-dir"
	installName := filepath.Join(customDir, "installs")
	deploymentName := "nginx-deployment"

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
		By("Install test Manifest CR")
		testManifest := testutils.NewTestManifest("warning-check")
		manifestName := testManifest.GetName()
		validImageSpec := hlp.CreateOCIImageSpec(installName, hlp.Server.Listener.Addr().String(), false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())

		Expect(hlp.InstallManifest(testManifest, imageSpecByte, true)).To(Succeed())

		By("Ensure that deployment and Sample CR are deployed and ready")
		deploy := &appsv1.Deployment{}
		Eventually(setDeploymentStatus(deploymentName, deploy), standardTimeout, standardInterval).Should(Succeed())
		sampleCR := emptySampleCR(manifestName)
		Eventually(setCRStatus(sampleCR, declarative.StateReady), standardTimeout, standardInterval).Should(Succeed())

		By("Verify the Manifest CR is in the \"Ready\" state")
		Eventually(hlp.ExpectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("Verify manifest status list all resources correctly")
		status, err := hlp.GetManifestStatus(manifestName)
		Expect(err).ToNot(HaveOccurred())
		Expect(status.Synced).To(HaveLen(2))
		expectedDeployment := asResource(deploymentName, "default", "apps", "v1", "Deployment")
		expectedCRD := asResource("samples.operator.kyma-project.io", "",
			"apiextensions.k8s.io", "v1", "CustomResourceDefinition")
		Expect(status.Synced).To(ContainElement(expectedDeployment))
		Expect(status.Synced).To(ContainElement(expectedCRD))

		By("When the Module CR state is changed to \"Warning\"")
		Eventually(setCRStatus(sampleCR, declarative.StateWarning), standardTimeout, standardInterval).Should(Succeed())

		By("Verify the Manifest CR state also changes to \"Warning\"")
		Eventually(hlp.ExpectManifestStateIn(declarative.StateWarning), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("When the Module CR state is changed back to \"Ready\"")
		Eventually(setCRStatus(sampleCR, declarative.StateReady), standardTimeout, standardInterval).Should(Succeed())

		By("Verify the Manifest CR state changes back to \"Ready\"")
		Eventually(hlp.ExpectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).Should(BeTrue())
		Eventually(verifyObjectExists(sampleCR), standardTimeout, standardInterval).Should(BeTrue())

		Eventually(hlp.DeleteManifestAndVerify(testManifest), standardTimeout, standardInterval).Should(Succeed())

		By("verify target resources got deleted")
		Eventually(verifyObjectExists(sampleCR), standardTimeout, standardInterval).Should(BeFalse())
		Eventually(verifyObjectExists(expectedCRD.ToUnstructured()), standardTimeout, standardInterval).Should(BeFalse())
		Eventually(verifyObjectExists(expectedDeployment.ToUnstructured()), standardTimeout, standardInterval).
			Should(BeFalse())
	})
})

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

func emptySampleCR(manifestName string) *unstructured.Unstructured {
	res := &unstructured.Unstructured{}
	res.SetGroupVersionKind(
		schema.GroupVersionKind{Group: "operator.kyma-project.io", Version: "v1alpha1", Kind: "Sample"})
	res.SetName("sample-cr-" + manifestName)
	res.SetNamespace(metav1.NamespaceDefault)
	return res
}

func setCRStatus(moduleCR *unstructured.Unstructured, statusValue declarative.State) func() error {
	return func() error {
		err := hlp.K8sClient.Get(
			hlp.Ctx, client.ObjectKeyFromObject(moduleCR),
			moduleCR,
		)
		if err != nil {
			return err
		}
		if err = unstructured.SetNestedMap(moduleCR.Object, map[string]any{}, "status"); err != nil {
			return err
		}
		if err = unstructured.SetNestedField(moduleCR.Object, string(statusValue), "status", "state"); err != nil {
			return err
		}
		return hlp.K8sClient.Status().Update(hlp.Ctx, moduleCR)
	}
}

func setDeploymentStatus(name string, deploy *appsv1.Deployment) func() error {
	return func() error {
		err := hlp.K8sClient.Get(
			hlp.Ctx, client.ObjectKey{
				Namespace: metav1.NamespaceDefault,
				Name:      name,
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
}
