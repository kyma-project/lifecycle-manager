package v1beta1_test

import (
	"encoding/json"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
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

var _ = Describe("Custom Manifest consistency check", Ordered, func() {
	mainOciTempDir := "main-dir"
	installName := filepath.Join(mainOciTempDir, "installs")
	crdName := filepath.Join(mainOciTempDir, "crds")
	It(
		"setup OCI", func() {
			PushToRemoteOCIRegistry(installName, layerInstalls)
			PushToRemoteOCIRegistry(crdName, layerCRDs)
		},
	)
	BeforeAll(
		func() {
			Expect(os.RemoveAll(filepath.Join(os.TempDir(), mainOciTempDir))).To(Succeed())
		},
	)

	It("Install OCI manifest and wait until it's ready", func() {
		manifest := NewTestManifest("custom-check-oci")
		manifestName := manifest.GetName()
		validImageSpec := createOCIImageSpec(manifestName, server.Listener.Addr().String(), layerInstalls)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		Expect(installManifest(manifest, imageSpecByte, true)).To(Succeed())
		Eventually(expectManifestStateIn(declarative.StateReady), standardTimeout, standardInterval).
			WithArguments(manifestName).Should(Succeed())
		Eventually(expectHelmClientCacheExist(true), standardTimeout, standardInterval).
			WithArguments(internalV1beta1.GenerateCacheKey(manifest.GetLabels()[v1beta1.KymaName],
				strconv.FormatBool(manifest.Spec.Remote), manifest.GetNamespace())).Should(BeTrue())
		cacheKey := internalV1beta1.GenerateCacheKey(manifest.GetLabels()[v1beta1.KymaName],
			strconv.FormatBool(manifest.Spec.Remote), manifest.GetNamespace())

		cachedClient := reconciler.ClientCache.GetClientFromCache(cacheKey)

		By("Verifying that deployment and Sample CR are deployed and ready")
		deploy := &appsv1.Deployment{}
		sampleCR := &unstructured.Unstructured{}
		Expect(verifyDeploymentInstallation(deploy)).To(Succeed())
		Expect(verifySampleCRInstallation(sampleCR)).To(Succeed())
		By("Preparing resources for the custom readiness check")
		resources := make([]*resource.Info, 0)
		sampleCRInfo, err := cachedClient.ResourceInfo(sampleCR, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(sampleCRInfo).ToNot(BeNil())
		resources = append(resources, sampleCRInfo)
		deployUnstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployUnstructuredObj).ToNot(BeNil())
		deployUnstructured := &unstructured.Unstructured{}
		deployUnstructured.SetUnstructuredContent(deployUnstructuredObj)
		deployUnstructured.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		deployInfo, err := cachedClient.ResourceInfo(deployUnstructured, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(deployInfo).ToNot(BeNil())
		resources = append(resources, deployInfo)
		By("Executing the custom readiness check")
		customReadyCheck := internalV1beta1.NewManifestCustomResourceReadyCheck()
		Expect(customReadyCheck.Run(ctx, cachedClient, manifest, resources)).To(Succeed())
	})
})

func verifyDeploymentInstallation(deploy *appsv1.Deployment) error {
	err := k8sClient.Get(
		ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      "busybox-deploy",
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

func verifySampleCRInstallation(sampleCR *unstructured.Unstructured) error {
	sampleCR.SetGroupVersionKind(v1beta1.GroupVersion.WithKind("SampleCRD"))
	err := k8sClient.Get(
		ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      "sample-crd-from-manifest",
		}, sampleCR,
	)
	if err != nil {
		return err
	}
	err = unstructured.SetNestedField(sampleCR.Object, "Ready", "status", "state")
	if err != nil {
		return err
	}
	err = k8sClient.Status().Update(ctx, sampleCR)
	if err != nil {
		return err
	}
	return nil
}
