package v1beta1_test

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	internalV1beta1 "github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const nginxControllerDeploymentSuffix = "nginx-ingress-controller"

var _ = Describe("Custom Manifest consistency check, given Manifest CR with Helm specs", Ordered, func() {
	setHelmEnv()
	validHelmChartSpec := v1beta2.HelmChartSpec{
		ChartName: "nginx-ingress",
		URL:       "https://helm.nginx.com/stable",
		Type:      "helm-chart",
	}

	validHelmChartSpecBytes, err := json.Marshal(validHelmChartSpec)
	Expect(err).NotTo(HaveOccurred())

	It("Install nginx helm chart", func() {
		manifest := NewTestManifest("custom-check-helm")
		manifestName := manifest.GetName()
		Eventually(addInstallSpec(validHelmChartSpecBytes), standardTimeout, standardInterval).
			WithArguments(manifest).Should(Succeed())
		Eventually(expectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())
		cacheKey := internalV1beta1.GenerateCacheKey(manifest.GetLabels()[v1beta2.KymaName],
			strconv.FormatBool(manifest.Spec.Remote), manifest.GetNamespace())
		cachedClient := reconciler.ClientCache.GetClientFromCache(cacheKey)

		By("Verifying that deployment and Sample CR are deployed and ready")
		deploy := &appsv1.Deployment{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())

		By("Preparing resources for the custom readiness check")
		resources, err := prepareResourceInfosForCustomCheck(cachedClient, deploy)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources).ToNot(BeEmpty())

		By("Executing the custom readiness check")
		customReadyCheck := internalV1beta1.NewManifestCustomResourceReadyCheck()
		Expect(customReadyCheck.Run(ctx, cachedClient, manifest, resources)).To(Succeed())

		By("cleaning up the manifest")
		Eventually(deleteManifestAndVerify(manifest), standardTimeout, standardInterval).Should(Succeed())
	})
})

func verifyDeploymentInstallation(deploy *appsv1.Deployment) error {
	err := k8sClient.Get(
		ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      fmt.Sprintf("%s-%s", manifestInstallName, nginxControllerDeploymentSuffix),
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
