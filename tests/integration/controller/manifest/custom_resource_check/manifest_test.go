package custom_resource_check_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning state propagation test", Ordered, func() {
	customDir := "custom-dir"
	installName := filepath.Join(customDir, "installs")
	deploymentName := "nginx-deployment"

	It(
		"setup OCI", func() {
			err := PushToRemoteOCIRegistry(server, manifestFilePath, installName)
			Expect(err).NotTo(HaveOccurred())
		},
	)
	BeforeEach(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), customDir))).To(Succeed())
		},
	)
	It("Install OCI specs including an nginx deployment", func() {
		By("Install test Manifest CR")
		testManifest, kyma := NewTestManifestWithParentKyma("warning-check")
		Eventually(CreateCR, standardTimeout, standardInterval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).
			Should(Succeed())
		Eventually(AddManifestToKymaStatus, standardTimeout, standardInterval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), testManifest.Name).
			Should(Succeed())

		manifestName := testManifest.GetName()
		validImageSpec, err := CreateOCIImageSpecFromFile(
			installName,
			server.Listener.Addr().String(),
			manifestFilePath,
		)
		Expect(err).NotTo(HaveOccurred())
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())

		Expect(InstallManifest(ctx, kcpClient, testManifest, imageSpecByte, true)).To(Succeed())

		By("Ensure that deployment and Sample CR are deployed and ready")
		deploy := &apiappsv1.Deployment{}
		Eventually(setDeploymentStatus(ctx, kcpClient, deploymentName, deploy), standardTimeout,
			standardInterval).Should(Succeed())
		sampleCR := emptySampleCR(manifestName)
		Eventually(setCRStatus(ctx, kcpClient, sampleCR, shared.StateReady), standardTimeout,
			standardInterval).Should(Succeed())

		By("Verify the Manifest CR is in the \"Ready\" state")
		Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady), standardTimeout,
			standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("Verify manifest status list all resources correctly")
		status, err := GetManifestStatus(ctx, kcpClient, manifestName)
		Expect(err).ToNot(HaveOccurred())
		Expect(status.Synced).To(HaveLen(2))
		expectedDeployment := asResource(deploymentName, "default", "apps", "v1", "Deployment")
		expectedCRD := asResource("samples.operator.kyma-project.io", "",
			"apiextensions.k8s.io", "v1", "CustomResourceDefinition")
		Expect(status.Synced).To(ContainElement(expectedDeployment))
		Expect(status.Synced).To(ContainElement(expectedCRD))

		By("When the Module CR state is changed to \"Warning\"")
		Eventually(setCRStatus(ctx, kcpClient, sampleCR, shared.StateWarning), standardTimeout,
			standardInterval).Should(Succeed())

		By("Verify the Manifest CR stays in the \"Ready\" state")
		Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady), standardTimeout,
			standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("When the Module CR state is changed back to \"Ready\"")
		Eventually(setCRStatus(ctx, kcpClient, sampleCR, shared.StateReady), standardTimeout,
			standardInterval).Should(Succeed())

		By("Verify the Manifest CR stays in the \"Ready\"")
		Eventually(ExpectManifestStateIn(ctx, kcpClient, shared.StateReady), standardTimeout,
			standardInterval).
			WithArguments(manifestName).Should(Succeed())

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(ctx, kcpClient, expectedDeployment.ToUnstructured()), standardTimeout,
			standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(ctx, kcpClient, expectedCRD.ToUnstructured()), standardTimeout,
			standardInterval).Should(BeTrue())
		Eventually(verifyObjectExists(ctx, kcpClient, sampleCR), standardTimeout,
			standardInterval).Should(BeTrue())

		Eventually(DeleteManifestAndVerify(ctx, kcpClient, testManifest), standardTimeout,
			standardInterval).Should(Succeed())

		By("verify target resources got deleted")
		Eventually(verifyObjectExists(ctx, kcpClient, sampleCR), standardTimeout,
			standardInterval).Should(BeFalse())
		Eventually(verifyObjectExists(ctx, kcpClient, expectedCRD.ToUnstructured()), standardTimeout,
			standardInterval).Should(BeFalse())
		Eventually(verifyObjectExists(ctx, kcpClient, expectedDeployment.ToUnstructured()), standardTimeout,
			standardInterval).
			Should(BeFalse())
	})
})

func asResource(name, namespace, group, version, kind string) shared.Resource {
	return shared.Resource{
		Name: name, Namespace: namespace,
		GroupVersionKind: apimetav1.GroupVersionKind{
			Group: group, Version: version, Kind: kind,
		},
	}
}

func verifyObjectExists(ctx context.Context, clnt client.Client, obj *unstructured.Unstructured) func() (bool, error) {
	return func() (bool, error) {
		err := clnt.Get(
			ctx, client.ObjectKeyFromObject(obj),
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
		schema.GroupVersionKind{
			Group:   templatev1alpha1.GroupVersion.Group,
			Version: templatev1alpha1.GroupVersion.Version,
			Kind:    string(templatev1alpha1.SampleKind),
		})
	res.SetName("sample-cr-" + manifestName)
	res.SetNamespace(apimetav1.NamespaceDefault)
	return res
}

func setCRStatus(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured,
	statusValue shared.State,
) func() error {
	return func() error {
		err := clnt.Get(
			ctx, client.ObjectKeyFromObject(moduleCR),
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
		return clnt.Status().Update(ctx, moduleCR)
	}
}

func setDeploymentStatus(ctx context.Context, clnt client.Client, name string,
	deploy *apiappsv1.Deployment,
) func() error {
	return func() error {
		err := clnt.Get(
			ctx, client.ObjectKey{
				Namespace: apimetav1.NamespaceDefault,
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
			apiappsv1.DeploymentCondition{
				Type:   apiappsv1.DeploymentAvailable,
				Status: apicorev1.ConditionTrue,
			},
			apiappsv1.DeploymentCondition{
				Type:   apiappsv1.DeploymentProgressing,
				Status: apicorev1.ConditionTrue,
				Reason: statecheck.NewReplicaSetAvailableReason,
			})
		err = clnt.Status().Update(ctx, deploy)
		if err != nil {
			return err
		}
		return nil
	}
}
