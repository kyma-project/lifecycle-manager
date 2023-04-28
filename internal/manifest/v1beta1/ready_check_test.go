package v1beta1_test

import (
	"context"
	"encoding/json"
	"path/filepath"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	internalV1beta1 "github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const nginxControllerDeploymentSuffix = "nginx-ingress-controller"

var _ = Describe("Custom Manifest consistency check, given Manifest CR with OCI specs", Ordered, func() {
	It("Install OCI specs including an nginx deployment", func() {
		manifest := NewTestManifest("custom-check-oci")
		manifestName := manifest.GetName()
		mainOciTempDir := "main-dir"
		installName := filepath.Join(mainOciTempDir, "installs")
		validImageSpec := createOCIImageSpec(installName, server.Listener.Addr().String())
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(installManifest(manifest, imageSpecByte, false)).To(Succeed())

		Eventually(expectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())

		testClient, err := declarativeTestClient(ctx, manifest)
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that deployment and Sample CR are deployed and ready")
		deploy := &appsv1.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Preparing resources for the custom readiness check")
		resources, err := prepareResourceInfosForCustomCheck(testClient, deploy)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).ToNot(BeEmpty())

		By("Executing the custom readiness check")
		customReadyCheck := internalV1beta1.NewManifestCustomResourceReadyCheck()
		Expect(customReadyCheck.Run(ctx, testClient, manifest, resources)).To(Succeed())

		By("cleaning up the manifest")
		Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
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

func declarativeTestClient(ctx context.Context, obj declarative.Object) (declarative.Client, error) {
	var err error

	cluster := &declarative.ClusterInfo{
		Config: cfg,
		Client: k8sClient,
	}

	cluster, err = targetClusterFn(ctx, obj)
	if err != nil {
		return nil, err
	}

	clt, err := declarative.NewSingletonClients(cluster, log.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	clt.Install().Atomic = false
	clt.Install().Replace = true
	clt.Install().DryRun = true
	clt.Install().IncludeCRDs = false
	clt.Install().CreateNamespace = false
	clt.Install().UseReleaseName = false
	clt.Install().IsUpgrade = true
	clt.Install().DisableHooks = true
	clt.Install().DisableOpenAPIValidation = true
	if clt.Install().Version == "" && clt.Install().Devel {
		clt.Install().Version = ">0.0.0-0"
	}
	return clt, nil
}
