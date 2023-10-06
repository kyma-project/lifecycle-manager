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

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errKymaNotInExpectedState = errors.New("kyma CR not in expected state")
	errModuleNotExisting      = errors.New("module does not exists in KymaCR")
	errKymaNotDeleted         = errors.New("kyma CR not deleted")
	errNoManifestFound        = errors.New("no manifest found")
	errUnexpectedState        = errors.New("unexpected state found for module")
	errModuleNotFound         = errors.New("module not found")
)

const (
	localHostname         = "0.0.0.0"
	k3dHostname           = "host.k3d.internal"
	defaultRemoteKymaName = "default"
	timeout               = 10 * time.Second
	interval              = 1 * time.Second
	remoteNamespace       = "kyma-system"
)

func CheckKymaIsInState(ctx context.Context,
	kymaName, kymaNamespace string,
	k8sClient client.Client,
	expectedState v1beta2.State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	GinkgoWriter.Printf("kyma %v\n", kyma)
	if kyma.Status.State != expectedState {
		logmsg, err := getManifestCRs(ctx, k8sClient)
		if err != nil {
			return fmt.Errorf("error getting manifest crs %w", err)
		}
		return fmt.Errorf("%w: expect %s, but in %s. Kyma CR: %#v, Manifest CRs: %s",
			errKymaNotInExpectedState, expectedState, kyma.Status.State, kyma, logmsg)
	}
	return nil
}

func getManifestCRs(ctx context.Context, k8sClient client.Client) (string, error) {
	manifests := &v1beta2.ManifestList{}
	if err := k8sClient.List(ctx, manifests); err != nil {
		return "", err
	}
	logmsg := ""
	for _, m := range manifests.Items {
		logmsg += fmt.Sprintf("Manifest CR: %#v", m)
	}
	return logmsg, nil
}

func CreateKymaSecret(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	patchedRuntimeConfig := strings.ReplaceAll(string(*runtimeConfig), localHostname, k3dHostname)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
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
	kymaNamespace string, wantedModules []v1beta2.Module, k8sClient client.Client, expectedState v1beta2.State,
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
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, secret)
	if util.IsNotFound(err) {
		return nil
	}
	Expect(err).ToNot(HaveOccurred())
	return k8sClient.Delete(ctx, secret)
}

func CheckKCPKymaCRDeleted(ctx context.Context,
	kymaName string, kymaNamespace string, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if util.IsNotFound(err) {
		return nil
	}
	return errKymaNotDeleted
}

func EnableModule(ctx context.Context,
	kymaName, kymaNamespace, moduleName, moduleChannel string,
	k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	GinkgoWriter.Printf("kyma %v\n", kyma)
	kyma.Spec.Modules = append(kyma.Spec.Modules, v1beta2.Module{
		Name:    moduleName,
		Channel: moduleChannel,
	})
	return k8sClient.Update(ctx, kyma)
}

func DisableModule(ctx context.Context, kymaName, kymaNamespace, moduleName string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	GinkgoWriter.Printf("kyma %v\n", kyma)

	for i, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			kyma.Spec.Modules = removeModuleWithIndex(kyma.Spec.Modules, i)
			break
		}
	}
	return k8sClient.Update(ctx, kyma)
}

func removeModuleWithIndex(s []v1beta2.Module, index int) []v1beta2.Module {
	return append(s[:index], s[index+1:]...)
}

func GetKymaStateMetricCount(ctx context.Context, kymaName, state string) (int, error) {
	clnt := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:9081/metrics", nil)
	if err != nil {
		return 0, err
	}
	response, err := clnt.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, err
	}
	bodyString := string(bodyBytes)

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

func GetManifestObjectKey(ctx context.Context, k8sClient client.Client, kymaName, templateName string) (
	types.NamespacedName, error,
) {
	manifests := &v1beta2.ManifestList{}
	if err := k8sClient.List(ctx, manifests); err != nil {
		return types.NamespacedName{Namespace: "", Name: ""}, fmt.Errorf("manifest fetching failed: %w", err)
	}
	for _, m := range manifests.Items {
		if ownerKyma, exists := m.Labels["operator.kyma-project.io/kyma-name"]; exists && ownerKyma == kymaName {
			if strings.Contains(m.Name, templateName) {
				return client.ObjectKeyFromObject(&m), nil
			}
		}
	}
	return types.NamespacedName{Namespace: "", Name: ""},
		fmt.Errorf("manifest fetching failed: %w", errNoManifestFound)
}

func DeleteManifest(ctx context.Context, objKey types.NamespacedName, k8sClient client.Client) error {
	manifest := &v1beta2.Manifest{}
	err := k8sClient.Get(ctx, objKey, manifest)
	if util.IsNotFound(err) {
		return nil
	}
	Expect(err).ToNot(HaveOccurred())
	return k8sClient.Delete(ctx, manifest)
}

func AddFinalizerToManifest(ctx context.Context,
	objKey types.NamespacedName,
	k8sClient client.Client,
	finalizer string,
) error {
	manifest := &v1beta2.Manifest{}
	if err := k8sClient.Get(ctx, objKey, manifest); err != nil {
		return err
	}
	GinkgoWriter.Printf("manifest %v\n", manifest)
	manifest.Finalizers = append(manifest.Finalizers, finalizer)
	err := k8sClient.Patch(ctx, manifest, client.Apply, client.ForceOwnership, client.FieldOwner("lifecycle-manager"))
	if err != nil {
		return err
	}
	return nil
}

func removeFromSlice(slice []string, element string) []string {
	for i := range slice {
		if slice[i] == element {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func RemoveFinalizerToManifest(ctx context.Context,
	objKey types.NamespacedName,
	k8sClient client.Client,
	finalizer string,
) error {
	manifest := &v1beta2.Manifest{}
	if err := k8sClient.Get(ctx, objKey, manifest); err != nil {
		return err
	}
	GinkgoWriter.Printf("manifest %v\n", manifest)
	manifest.Finalizers = removeFromSlice(manifest.Finalizers, finalizer)
	err := k8sClient.Patch(ctx, manifest, client.Apply, client.ForceOwnership, client.FieldOwner("lifecycle-manager"))
	if err != nil {
		return err
	}
	return nil
}

func CheckKymaModuleIsInState(ctx context.Context,
	kymaName, kymaNamespace string,
	k8sClient client.Client,
	moduleName string,
	expectedState v1beta2.State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return fmt.Errorf("error checking kyma module state: %w", err)
	}
	GinkgoWriter.Printf("kyma %v\n", kyma)
	for _, module := range kyma.Status.Modules {
		if module.Name == moduleName {
			if module.State == expectedState {
				return nil
			} else {
				return fmt.Errorf("error checking kyma module state: %w: state - %s module - %s",
					errUnexpectedState, module.State, moduleName)
			}
		}
	}
	return fmt.Errorf("error checking kyma module state: %w", errModuleNotFound)
}
