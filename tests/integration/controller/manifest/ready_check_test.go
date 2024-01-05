package manifest_test

import (
	"context"
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
			testutils.PushToRemoteOCIRegistry(server, manifestFilePath, installName)
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
		validImageSpec := testutils.CreateOCIImageSpec(installName, server.Listener.Addr().String(), manifestFilePath,
			false)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(testutils.InstallManifest(ctx, controlPlaneClient, testManifest, imageSpecByte, false)).To(Succeed())

		Eventually(testutils.ExpectManifestStateIn(ctx, controlPlaneClient, shared.StateReady), standardTimeout,
			standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient(controlPlaneClient)
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment is deployed and ready")
		deploy := &apiappsv1.Deployment{}
		Expect(verifyDeploymentInstallation(ctx, controlPlaneClient, deploy)).To(Succeed())

		By("Verifying manifest status contains all resources")
		status, err := testutils.GetManifestStatus(ctx, controlPlaneClient, manifestName)
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
		stateInfo, err := customReadyCheck.Run(ctx, testClient, testManifest, resources)
		Expect(err).NotTo(HaveOccurred())
		Expect(stateInfo.State).To(Equal(shared.StateReady))

		By("cleaning up the manifest")
		Eventually(verifyObjectExists(ctx, controlPlaneClient, expectedDeployment.ToUnstructured()), standardTimeout,
			standardInterval).
			Should(BeTrue())
		Eventually(verifyObjectExists(ctx, controlPlaneClient, expectedCRD.ToUnstructured()), standardTimeout,
			standardInterval).Should(BeTrue())

		Eventually(testutils.DeleteManifestAndVerify(ctx, controlPlaneClient, testManifest), standardTimeout,
			standardInterval).Should(Succeed())

		Eventually(verifyObjectExists(ctx, controlPlaneClient, expectedDeployment.ToUnstructured()), standardTimeout,
			standardInterval).
			Should(BeFalse())
		Eventually(verifyObjectExists(ctx, controlPlaneClient, expectedCRD.ToUnstructured()), standardTimeout,
			standardInterval).
			Should(BeFalse())
	})
})

func verifyDeploymentInstallation(ctx context.Context, clnt client.Client, deploy *apiappsv1.Deployment) error {
	err := clnt.Get(
		ctx, client.ObjectKey{
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
	err = clnt.Status().Update(ctx, deploy)
	if err != nil {
		return err
	}
	return nil
}

func prepareResourceInfosForCustomCheck(clt declarativev2.Client, deploy *apiappsv1.Deployment) ([]*resource.Info,
	error,
) {
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

func declarativeTestClient(clnt client.Client) (declarativev2.Client, error) {
	cluster := &declarativev2.ClusterInfo{
		Config: cfg,
		Client: clnt,
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
