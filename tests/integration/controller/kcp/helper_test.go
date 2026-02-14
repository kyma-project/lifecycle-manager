package kcp_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrExpectedLabelNotReset    = errors.New("expected label not reset")
	ErrWatcherLabelMissing      = errors.New("watcher label missing")
	ErrWatcherAnnotationMissing = errors.New("watcher annotation missing")
	ErrGlobalChannelMisMatch    = errors.New("kyma global channel mismatch")
)

// For EventFilters tests.
var (
	errKymaNotInExpectedState   = errors.New("kyma is not in expected state")
	errKymaNotInExpectedChannel = errors.New("kyma doesn't have expected channel")
	errKymaNotHaveExpectedLabel = errors.New("kyma doesn't have expected label")
)

const (
	InitSpecKey   = "initKey"
	InitSpecValue = "initValue"
)

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, kcpClient, kyma)
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})
}

// Note: Uses simplified logic and deletes ALL versions of the ModuleTemplates
// configured in the Kyma spec.
// For precise deletion one has to find the correct ModuleTemplate instance based on
// the version and channel mapping specified in a corresponding ModuleReleaseMeta.
func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		allVersionsOfAModule := &v1beta2.ModuleTemplateList{}
		listOpts := []client.ListOption{
			client.InNamespace(ControlPlaneNamespace),
			client.MatchingLabels{shared.ModuleName: module.Name},
		}
		Expect(kcpClient.List(ctx, allVersionsOfAModule, listOpts...)).Should(Succeed())
		for _, mtInstance := range allVersionsOfAModule.Items {
			Eventually(DeleteCR, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, &mtInstance).Should(Succeed())
		}
	}
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		moduleTemplateName := module.Name
		if module.Version != "" {
			moduleTemplateName = fmt.Sprintf("%s-%s", module.Name, module.Version)
		}

		template := builder.NewModuleTemplateBuilder().
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithName(moduleTemplateName).
			Build()
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
	}
}

func kymaChannelMatch(clnt client.Client, name, namespace, channel string) error {
	kyma, err := GetKyma(ctx, clnt, name, namespace)
	if err != nil {
		return err
	}
	if kyma.Spec.Channel != channel {
		return ErrGlobalChannelMisMatch
	}
	return nil
}

func watcherLabelsAnnotationsExist(clnt client.Client, remoteKyma *v1beta2.Kyma, kyma *v1beta2.Kyma,
	remoteSyncNamespace string,
) error {
	remoteKyma, err := GetKyma(ctx, clnt, remoteKyma.GetName(), remoteSyncNamespace)
	if err != nil {
		return err
	}
	if remoteKyma.Labels[shared.WatchedByLabel] != shared.WatchedByLabelValue {
		return ErrWatcherLabelMissing
	}
	if remoteKyma.Annotations[shared.OwnedByAnnotation] != fmt.Sprintf(shared.OwnedByFormat,
		kyma.GetNamespace(), kyma.GetName()) {
		return ErrWatcherAnnotationMissing
	}
	return nil
}

func expectModuleTemplateSpecGetReset(
	clnt client.Client,
	module v1beta2.Module,
	kyma *v1beta2.Kyma,
) error {
	moduleTemplate, _, err := GetModuleTemplateInfo(ctx, clnt, module, kyma)
	if err != nil {
		return err
	}
	if moduleTemplate.Spec.Data == nil {
		return ErrManifestResourceIsNil
	}
	initKey, found := moduleTemplate.Spec.Data.Object["spec"]
	if !found {
		return ErrExpectedLabelNotReset
	}
	initKeyM, mapOk := initKey.(map[string]any)
	if !mapOk {
		return ErrExpectedLabelNotReset
	}
	value, found := initKeyM[InitSpecKey]
	if !found {
		return ErrExpectedLabelNotReset
	}
	sValue, ok := value.(string)
	if !ok {
		return ErrExpectedLabelNotReset
	}
	if sValue != InitSpecValue {
		return ErrExpectedLabelNotReset
	}
	return nil
}

func kymaHasCondition(
	clnt client.Client,
	conditionType v1beta2.KymaConditionType,
	reason string,
	status apimetav1.ConditionStatus,
	kymaName,
	kymaNamespace string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	for _, cnd := range kyma.Status.Conditions {
		if cnd.Type == string(conditionType) && cnd.Reason == reason && cnd.Status == status {
			return nil
		}
	}

	return ErrNotContainsExpectedCondition
}

func containsModuleTemplateCondition(clnt client.Client, kymaName, kymaNamespace string) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	if !kyma.ContainsCondition(v1beta2.ConditionTypeModuleCatalog) {
		return ErrNotContainsExpectedCondition
	}
	return nil
}

func updateKymaCRD(clnt client.Client) (*apiextensionsv1.CustomResourceDefinition, error) {
	return updateCRDPropertyDescription(clnt, shared.KymaKind, "channel", "test change")
}

func updateModuleReleaseMetaCRD(clnt client.Client) (*apiextensionsv1.CustomResourceDefinition, error) {
	return updateCRDPropertyDescription(clnt, shared.ModuleReleaseMetaKind, "channels", "test change")
}

func updateCRDPropertyDescription(clnt client.Client, crdKind shared.Kind,
	propertyName, newValue string,
) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd, err := fetchCrd(clnt, crdKind)
	if err != nil {
		return nil, err
	}

	crd.SetManagedFields(nil)
	crdSpecVersions := crd.Spec.Versions
	channelProperty := getCrdSpec(crd).Properties[propertyName]
	channelProperty.Description = newValue
	getCrdSpec(crd).Properties[propertyName] = channelProperty
	crd.Spec = apiextensionsv1.CustomResourceDefinitionSpec{
		Versions:              crdSpecVersions,
		Names:                 crd.Spec.Names,
		Group:                 crd.Spec.Group,
		Conversion:            crd.Spec.Conversion,
		Scope:                 crd.Spec.Scope,
		PreserveUnknownFields: crd.Spec.PreserveUnknownFields,
	}
	if err := clnt.Patch(ctx, crd,
		client.Apply,
		client.ForceOwnership,
		fieldowners.LifecycleManager); err != nil {
		return nil, err
	}
	crd, err = fetchCrd(clnt, crdKind)
	crdName := fmt.Sprintf("%s.%s", crdKind.Plural(), v1beta2.GroupVersion.Group)

	// Replace the cached CRD after updating the KCP CRD to validate that
	// the Generation values are updated correctly
	if _, ok := crdCache.Get(crdName); ok {
		crdCache.Add(crdName, *crd)
	}
	if err != nil {
		return nil, err
	}
	return crd, nil
}

func getCrdSpec(crd *apiextensionsv1.CustomResourceDefinition) apiextensionsv1.JSONSchemaProps {
	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
}

func fetchCrd(clnt client.Client, crdKind shared.Kind) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := clnt.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", crdKind.Plural(), v1beta2.GroupVersion.Group),
		}, crd,
	); err != nil {
		return nil, err
	}

	return crd, nil
}

// Helpers for EventFilters tests.
func updateKymaChannel(ctx context.Context,
	k8sClient client.Client,
	kymaName string,
	kymaNamespace string,
	channel string,
) error {
	kyma := &v1beta2.Kyma{}
	// Get the latest version of the Kyma resource
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	kyma.Spec.Channel = channel

	return updateKyma(ctx, k8sClient, kyma)
}

func addLabelToKyma(ctx context.Context,
	k8sClient client.Client,
	kyma *v1beta2.Kyma,
	labelKey, labelValue string,
) error {
	if kyma.Labels == nil {
		kyma.Labels = make(map[string]string)
	}
	kyma.Labels[labelKey] = labelValue

	return updateKyma(ctx, k8sClient, kyma)
}

func kymaIsInExpectedStateWithUpdatedChannel(k8sClient client.Client,
	kymaName string,
	kymaNamespace string,
	expectedChannel string,
	expectedState shared.State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	if kyma.Status.ActiveChannel != expectedChannel {
		return fmt.Errorf("%w: expected channel: %s, but found: %s",
			errKymaNotInExpectedChannel, expectedChannel, kyma.Spec.Channel)
	}

	return validateKymaStatus(kyma.Status.State, expectedState)
}

func kymaIsInExpectedStateWithLabelUpdated(k8sClient client.Client,
	kymaName string,
	kymaNamespace string,
	expectedLabelKey string,
	expectedLabelValue string,
	expectedState shared.State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	if kyma.Labels[expectedLabelKey] != expectedLabelValue {
		return fmt.Errorf("%w: expected label value: %s, but found: %s",
			errKymaNotHaveExpectedLabel, expectedLabelValue, kyma.Labels[expectedLabelKey])
	}

	return validateKymaStatus(kyma.Status.State, expectedState)
}

func validateKymaStatus(kymaState, expectedState shared.State) error {
	if kymaState != expectedState {
		return fmt.Errorf("%w: expected state: %s, but found: %s",
			errKymaNotInExpectedState, expectedState, kymaState)
	}

	return nil
}

func updateKyma(ctx context.Context, k8sClient client.Client, kyma *v1beta2.Kyma) error {
	err := k8sClient.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("failed to update Kyma with error %w", err)
	}
	return nil
}
