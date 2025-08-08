package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
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
	ModuleVersionToBeUsed   = "1.0.3"
	NewerVersion            = "2.4.2-e2e-test"
	MisconfiguredModuleName = "template-operator-misconfigured"
)

func InitEmptyKymaBeforeAll(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		By("When a KCP Kyma CR is created on the KCP cluster")
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient).
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
			Eventually(CheckPodLogs).
				WithContext(ctx).
				WithArguments(RemoteNamespace, skrwebhookresources.SkrResourceName, "server",
					"Starting server for validation endpoint", skrRESTConfig,
					skrClient, &apimetav1.Time{Time: time.Now().Add(-5 * time.Minute)}).
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

func CreateKymaSecret(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	patchedRuntimeConfig := strings.ReplaceAll(string(*skrConfig), localHostname, skrHostname)
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
			Labels: map[string]string{
				shared.KymaName: kymaName,
			},
		},
		Data: map[string][]byte{"config": []byte(patchedRuntimeConfig)},
	}
	return k8sClient.Create(ctx, secret)
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

func DeleteKymaSecret(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	secret := &apicorev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, secret)
	if util.IsNotFound(err) {
		return nil
	}
	Expect(err).ToNot(HaveOccurred())
	return k8sClient.Delete(ctx, secret)
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
