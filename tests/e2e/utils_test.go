package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	errKymaNotInExpectedState   = errors.New("kyma CR not in expected state")
	errModuleNotExisting        = errors.New("module does not exists in KymaCR")
	errLabelNotExistOnNamespace = errors.New("label does not exist on namespace")
	ErrLabelNotExistOnCR        = errors.New("label does not exist on CustomResource")
)

const (
	localHostname           = "0.0.0.0"
	skrHostname             = "skr.cluster.local"
	defaultRemoteKymaName   = "default"
	EventuallyTimeout       = 10 * time.Second
	ConsistentDuration      = 20 * time.Second
	interval                = 500 * time.Millisecond
	moduleCRFinalizer       = "cr-finalizer"
	e2eVersionSuffix        = "-e2e"
	smokeTestVersionSuffix  = "-smoke-test"
	MisconfiguredModuleName = "template-operator-misconfigured"
	// GlobalAccountID1 is used to test uninstallation when the Kyma's
	// global-account-id no longer matches the deployer module's kymaSelector.
	GlobalAccountID1 = "a1c1d2e3-4a5b-6c7d-8e9f-0a1b2c3d4e5f"
	GlobalAccountID2 = "f6e5d4c3-b2a1-9087-6543-210fedcba987"
)

var (
	// OlderVersion and NewerVersion are read from tests/e2e/versions.yaml to stay
	// in sync with the versions deployed by the Makefile test setup.
	OlderVersion                = readE2EVersionFromFile("module-version-older") + e2eVersionSuffix
	NewerVersion                = readE2EVersionFromFile("module-version-newer") + e2eVersionSuffix
	MandatoryModuleOlderVersion = readE2EVersionFromFile("module-version-older") + smokeTestVersionSuffix
	MandatoryModuleNewerVersion = readE2EVersionFromFile("module-version-newer") + smokeTestVersionSuffix
	ModuleVersionToBeUsed       = mustReadTemplateOperatorVersion()
)

func mustReadTemplateOperatorVersion() string {
	content, err := os.ReadFile("../../versions.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to read versions.yaml: %v", err))
	}
	var versions map[string]string
	if err := yaml.Unmarshal(content, &versions); err != nil {
		panic(fmt.Sprintf("failed to parse versions.yaml: %v", err))
	}
	version, ok := versions["template-operator"]
	if !ok {
		panic("template-operator version not found in versions.yaml")
	}
	return version
}

func readE2EVersionFromFile(key string) string {
	content, err := os.ReadFile("versions.yaml")
	if err != nil {
		panic(fmt.Sprintf("failed to read versions.yaml: %v", err))
	}
	var versions map[string]string
	if err := yaml.Unmarshal(content, &versions); err != nil {
		panic(fmt.Sprintf("failed to parse versions.yaml: %v", err))
	}
	v, ok := versions[key]
	if !ok {
		panic(fmt.Sprintf("%q not found in versions.yaml", key))
	}
	return v
}

func InitEmptyKymaBeforeAll(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		By("When a KCP Kyma CR is created on the KCP cluster")
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), string(*skrConfig)).
			Should(Succeed())
		Eventually(kcpClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("Then the Kyma CR is in a \"Ready\" State on the KCP cluster ")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
			Should(Succeed())
		By("And the Kyma CR is in \"Ready\" State on the SKR cluster")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(RemoteNamespace, []v1beta2.Module{}, skrClient, shared.StateReady).
			Should(Succeed())
		By("And Runtime Watcher deployment is up and running in SKR", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, skrwebhookresources.SkrResourceName,
					RemoteNamespace).
				Should(Succeed())
		})
	})
}

func CleanupKymaAfterAll(kyma *v1beta2.Kyma) {
	AfterAll(func() {
		By("When delete KCP Kyma")
		Eventually(DeleteKymaByForceRemovePurgeFinalizer).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).
			Should(Succeed())

		By("Then SKR Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), skrClient).
			Should(Succeed())
		By("Then KCP Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient).
			Should(Succeed())
	})
}

func CheckIfExists(ctx context.Context, name, namespace, group, version, kind string, clnt client.Client) error {
	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})

	err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, resourceCR)
	return CRExists(resourceCR, err)
}

func CreateKymaSecret(ctx context.Context, k8sClient client.Client, kymaName, runtimeConfig string) error {
	patchedRuntimeConfig := strings.ReplaceAll(runtimeConfig, localHostname, skrHostname)
	GinkgoWriter.Printf("CreateKymaSecret: %s\n", patchedRuntimeConfig)
	return CreateAccessSecret(ctx, k8sClient, kymaName, patchedRuntimeConfig)
}

func CreateInvalidKymaSecret(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	invalidRuntimeConfig := strings.ReplaceAll(string(*skrConfig), localHostname, "non.existent.url")
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
			Labels: map[string]string{
				shared.KymaName: kymaName,
			},
		},
		Data: map[string][]byte{"config": []byte(invalidRuntimeConfig)},
	}
	return k8sClient.Create(ctx, secret)
}

func CreateAnySecret(ctx context.Context, name string, clnt client.Client) error {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: shared.DefaultControlPlaneNamespace,
		},
		Data: map[string][]byte{"data": []byte(random.Name())},
	}
	return clnt.Create(ctx, secret)
}

func DeleteAnySecret(ctx context.Context, name string, clnt client.Client) error {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: shared.DefaultControlPlaneNamespace,
		},
	}
	return clnt.Delete(ctx, secret)
}

func CheckRemoteKymaCR(ctx context.Context,
	kymaNamespace string, wantedModules []v1beta2.Module, k8sClient client.Client, expectedState shared.State,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return err
	}

	for _, wantedModule := range wantedModules {
		exists := false
		for _, givenModule := range kyma.Spec.Modules {
			if givenModule.Name == wantedModule.Name &&
				givenModule.Channel == wantedModule.Channel {
				exists = true
				break
			}
		}
		if !exists {
			return fmt.Errorf("%w: %s/%s", errModuleNotExisting, wantedModule.Name, wantedModule.Channel)
		}
	}
	if kyma.Status.State != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			errKymaNotInExpectedState, expectedState, kyma.Status.State)
	}
	return nil
}

func EnsureNamespaceHasCorrectLabels(ctx context.Context, clnt client.Client, kymaNamespace string,
	labels map[string]string,
) error {
	var namespace apicorev1.Namespace
	if err := clnt.Get(ctx, client.ObjectKey{Name: kymaNamespace}, &namespace); err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", kymaNamespace, err)
	}

	if namespace.Labels == nil {
		return errLabelNotExistOnNamespace
	}

	for k, v := range labels {
		if namespace.Labels[k] != v {
			return fmt.Errorf("label %s has value %s, expected %s", k, namespace.Labels[k], v)
		}
	}

	return nil
}

func SetFinalizer(name, namespace, group, version, kind string, finalizers []string, clnt client.Client) error {
	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	if err := clnt.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return err
	}

	resourceCR.SetFinalizers(finalizers)
	return clnt.Update(ctx, resourceCR)
}

func CheckSampleCRIsInState(ctx context.Context, name, namespace string, clnt client.Client,
	expectedState shared.State,
) error {
	return CRIsInState(ctx,
		"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind),
		name, namespace,
		[]string{"status", "state"},
		clnt,
		expectedState)
}

func CheckSampleCRHasExpectedLabel(ctx context.Context, name, namespace string, clnt client.Client,
	labelKey, labelValue string,
) error {
	customResource, err := GetCR(ctx, clnt, client.ObjectKey{Name: name, Namespace: namespace}, schema.GroupVersionKind{
		Group:   templatev1alpha1.GroupVersion.Group,
		Version: templatev1alpha1.GroupVersion.Version,
		Kind:    string(templatev1alpha1.SampleKind),
	})
	if err != nil {
		return err
	}

	labels := customResource.GetLabels()
	if labels == nil || labels[labelKey] != labelValue {
		return ErrLabelNotExistOnCR
	}

	return nil
}

func DeploymentContainerHasFlag(ctx context.Context,
	deploymentName, namespace, flagName, flagValue string, clnt client.Client,
) error {
	klmDeployment, err := GetDeployment(ctx, clnt, deploymentName, namespace)
	if err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}

	for _, container := range klmDeployment.Spec.Template.Spec.Containers {
		for _, arg := range container.Args {
			if strings.Contains(arg, flagName) && strings.Contains(arg, flagValue) {
				return nil
			}
		}
	}
	return fmt.Errorf("flag %s with value %s not found in deployment %s", flagName, flagValue, deploymentName)
}

func DeploymentPodSpecHasImagePullSecret(ctx context.Context,
	deploymentName, namespace, secretName string, clnt client.Client,
) error {
	klmDeployment, err := GetDeployment(ctx, clnt, deploymentName, namespace)
	if err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}

	pullSecrets := klmDeployment.Spec.Template.Spec.ImagePullSecrets
	for _, pullSecret := range pullSecrets {
		if pullSecret.Name == secretName {
			return nil
		}
	}
	return fmt.Errorf("imagePullSecret %s not found in deployment %s", secretName, deploymentName)
}

func SecretDataEquals(
	ctx context.Context, clnt client.Client, name, namespace, dataKey string, expectedData []byte,
) error {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return err
	}
	actualData, ok := secret.Data[dataKey]
	if !ok {
		return fmt.Errorf("secret %s/%s does not have key %q", namespace, name, dataKey)
	}
	if !bytes.Equal(actualData, expectedData) {
		return fmt.Errorf("secret %s/%s key %q data does not match", namespace, name, dataKey)
	}
	return nil
}

func UpdateSecretLabel(
	ctx context.Context, clnt client.Client, name, namespace, labelKey, labelValue string,
) error {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return err
	}
	patch := client.MergeFrom(secret.DeepCopy())
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[labelKey] = labelValue
	return clnt.Patch(ctx, secret, patch)
}

func RemoveSecretLabel(
	ctx context.Context, clnt client.Client, name, namespace, labelKey string,
) error {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return err
	}
	patch := client.MergeFrom(secret.DeepCopy())
	delete(secret.Labels, labelKey)
	return clnt.Patch(ctx, secret, patch)
}

func UpdateSecretDataKey(
	ctx context.Context, clnt client.Client, name, namespace, dataKey string, data []byte,
) error {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		return err
	}
	patch := client.MergeFrom(secret.DeepCopy())
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[dataKey] = data
	return clnt.Patch(ctx, secret, patch)
}

func DeploymentContainersHaveImagePullSecretEnv(ctx context.Context,
	deploymentName, namespace, secretName string, clnt client.Client,
) error {
	klmDeployment, err := GetDeployment(ctx, clnt, deploymentName, namespace)
	if err != nil {
		return fmt.Errorf("could not get deployment: %w", err)
	}

	containers := klmDeployment.Spec.Template.Spec.Containers
	for _, container := range containers {
		found := false
		for _, envVar := range container.Env {
			if envVar.Name == render.SkrImagePullSecretEnvName && envVar.Value == secretName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("env var %s with value %s not found in container %s of deployment %s",
				render.SkrImagePullSecretEnvName, secretName, container.Name, deploymentName)
		}
	}
	return nil
}
