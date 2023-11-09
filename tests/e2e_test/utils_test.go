package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	errKymaNotInExpectedState       = errors.New("kyma CR not in expected state")
	errManifestNotInExpectedState   = errors.New("manifest CR not in expected state")
	errModuleNotExisting            = errors.New("module does not exists in KymaCR")
	errManifestDeletionTimestampSet = errors.New("manifest CR has set DeletionTimeStamp")
	errResourceExists               = errors.New("resource still exists")
)

const (
	localHostname         = "0.0.0.0"
	k3dHostname           = "host.k3d.internal"
	defaultRemoteKymaName = "default"
	timeout               = 10 * time.Second
	interval              = 1 * time.Second
	remoteNamespace       = "kyma-system"
	controlPlaneNamespace = "kcp-system"
	moduleCRFinalizer     = "cr-finalizer"
)

func InitEmptyKymaBeforeAll(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		By("When a KCP Kyma CR is created on the KCP cluster")
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("Then the Kyma CR is in a \"Ready\" State on the KCP cluster ")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
			Should(Succeed())
		By("And the Kyma CR is in \"Ready\" State on the SKR cluster")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, shared.StateReady).
			Should(Succeed())
	})
}

func CleanupKymaAfterAll(kyma *v1beta2.Kyma) {
	AfterAll(func() {
		By("When delete KCP Kyma")
		Eventually(DeleteKymaByForceRemovePurgeFinalizer).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).
			Should(Succeed())

		By("Then SKR Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), runtimeClient).
			Should(Succeed())
		By("Then KCP Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})
}

func CheckManifestIsInState(
	ctx context.Context,
	kymaName, kymaNamespace, moduleName string,
	clnt client.Client,
	expectedState shared.State,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}

	if manifest.Status.State != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			errManifestNotInExpectedState, expectedState, manifest.Status.State)
	}
	return nil
}

func ManifestNoDeletionTimeStampSet(ctx context.Context,
	kymaName, kymaNamespace, moduleName string,
	clnt client.Client,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}

	if !manifest.ObjectMeta.DeletionTimestamp.IsZero() {
		return errManifestDeletionTimestampSet
	}
	return nil
}

func CheckIfExists(ctx context.Context, name, namespace, group, version, kind string, clnt client.Client) error {
	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	return clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, resourceCR)
}

func CheckIfNotExists(ctx context.Context, name, namespace, group, version, kind string, clnt client.Client) error {
	resourceCR := &unstructured.Unstructured{}
	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, resourceCR)
	if util.IsNotFound(err) {
		return nil
	}
	return fmt.Errorf("%w: %s %s/%s should be deleted", errResourceExists, kind, namespace, name)
}

func CreateKymaSecret(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	patchedRuntimeConfig := strings.ReplaceAll(string(*runtimeConfig), localHostname, k3dHostname)
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
			Labels: map[string]string{
				v1beta2.KymaName:  kymaName,
				v1beta2.ManagedBy: v1beta2.OperatorName,
			},
		},
		Data: map[string][]byte{"config": []byte(patchedRuntimeConfig)},
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

func GetKymaStateMetricCount(ctx context.Context, kymaName, state string) (int, error) {
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(
		`lifecycle_mgr_kyma_state{instance_id="[^"]+",kyma_name="` + kymaName + `",shoot="[^"]+",state="` + state +
			`"} (\d+)`)
	match := re.FindStringSubmatch(bodyString)
	if len(match) > 1 {
		count, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, err
		}
		return count, nil
	}

	return 0, nil
}

func GetModuleStateMetricCount(ctx context.Context, kymaName, moduleName, state string) (int, error) {
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(
		`lifecycle_mgr_module_state{instance_id="[^"]+",kyma_name="` + kymaName +
			`",module_name="` + moduleName +
			`",shoot="[^"]+",state="` + state +
			`"} (\d+)`)
	match := re.FindStringSubmatch(bodyString)
	if len(match) > 1 {
		count, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, err
		}
		return count, nil
	}

	return 0, nil
}

func CheckSampleCRIsInState(ctx context.Context, name, namespace string, clnt client.Client,
	expectedState string,
) error {
	return CRIsInState(ctx,
		"operator.kyma-project.io", "v1alpha1", "Sample",
		name, namespace,
		[]string{"status", "state"},
		clnt,
		expectedState)
}

func getMetricsBody(ctx context.Context) (string, error) {
	clnt := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:9081/metrics", nil)
	if err != nil {
		return "", err
	}
	response, err := clnt.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)

	return bodyString, nil
}

func PurgeMetricsAreAsExpected(ctx context.Context,
	timeShouldBeMoreThan float64,
	expectedRequests int,
) bool {
	correctCount := false
	correctTime := false
	bodyString, err := getMetricsBody(ctx)
	if err != nil {
		return false
	}
	reg := regexp.MustCompile(`lifecycle_mgr_purgectrl_time ([0-9]*\.?[0-9]+)`)
	match := reg.FindStringSubmatch(bodyString)

	if len(match) > 1 {
		duration, err := strconv.ParseFloat(match[1], 64)
		if err == nil && duration > timeShouldBeMoreThan {
			correctTime = true
		}
	}

	reg = regexp.MustCompile(`lifecycle_mgr_purgectrl_requests_total (\d+)`)
	match = reg.FindStringSubmatch(bodyString)

	if len(match) > 1 {
		count, err := strconv.Atoi(match[1])
		if err == nil && count == expectedRequests {
			correctCount = true
		}
	}

	return correctTime && correctCount
}
