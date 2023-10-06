package control_plane_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/cache"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	compdesc2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrExpectedLabelNotReset    = errors.New("expected label not reset")
	ErrWatcherLabelMissing      = errors.New("watcher label missing")
	ErrWatcherAnnotationMissing = errors.New("watcher annotation missing")
	ErrGlobalChannelMisMatch    = errors.New("kyma global channel mismatch")
)

const (
	InitSpecKey   = "initKey"
	InitSpecValue = "initValue"
)

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma)
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma)
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdesc2.SchemaVersion).Build()
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
	}
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdesc2.SchemaVersion).Build()
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
	if remoteKyma.Labels[v1beta2.WatchedByLabel] != v1beta2.OperatorName {
		return ErrWatcherLabelMissing
	}
	if remoteKyma.Annotations[v1beta2.OwnedByAnnotation] != fmt.Sprintf(v1beta2.OwnedByFormat,
		kyma.GetNamespace(), kyma.GetName()) {
		return ErrWatcherAnnotationMissing
	}
	return nil
}

func expectModuleTemplateSpecGetReset(
	clnt client.Client,
	module v1beta2.Module,
	kymaChannel string,
) error {
	moduleTemplate, err := GetModuleTemplate(ctx, clnt, module, kymaChannel)
	if err != nil {
		return err
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
	status metav1.ConditionStatus,
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

func notContainsModuleInSpec(clnt client.Client, kymaName, kymaNamespace, moduleName string) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for _, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			return ErrContainsUnexpectedModules
		}
	}

	return nil
}

func containsModuleInSpec(clnt client.Client, kymaName, kymaNamespace, moduleName string) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for _, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			return nil
		}
	}

	return ErrNotContainsExpectedModules
}

func addModuleToKyma(clnt client.Client, kymaName, kymaNamespace string, module v1beta2.Module) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, module)
	return clnt.Update(ctx, kyma)
}

func removeModuleFromKyma(clnt client.Client, kymaName, kymaNamespace, moduleName string) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	for i, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			kyma.Spec.Modules = append(kyma.Spec.Modules[:i], kyma.Spec.Modules[i+1:]...)
			break
		}
	}
	return clnt.Update(ctx, kyma)
}

func updateKymaCRD(clnt client.Client) (*v1extensions.CustomResourceDefinition, error) {
	crd, err := fetchCrd(clnt, v1beta2.KymaKind)
	if err != nil {
		return nil, err
	}

	crd.SetManagedFields(nil)
	crdSpecVersions := crd.Spec.Versions
	channelProperty := getCrdSpec(crd).Properties["channel"]
	channelProperty.Description = "test change"
	getCrdSpec(crd).Properties["channel"] = channelProperty
	crd.Spec = v1extensions.CustomResourceDefinitionSpec{
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
		client.FieldOwner(v1beta2.OperatorName)); err != nil {
		return nil, err
	}
	crd, err = fetchCrd(clnt, v1beta2.KymaKind)
	kymaCrdName := fmt.Sprintf("%s.%s", v1beta2.KymaKind.Plural(), v1beta2.GroupVersion.Group)

	// Replace the cached CRD after updating the KCP CRD to validate that
	// the Generation values are updated correctly
	if _, ok := cache.GetCachedCRD(kymaCrdName); ok {
		cache.SetCRDInCache(kymaCrdName, *crd)
	}
	if err != nil {
		return nil, err
	}
	return crd, nil
}

func getCrdSpec(crd *v1extensions.CustomResourceDefinition) v1extensions.JSONSchemaProps {
	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
}

func fetchCrd(clnt client.Client, crdKind v1beta2.Kind) (*v1extensions.CustomResourceDefinition, error) {
	crd := &v1extensions.CustomResourceDefinition{}
	if err := clnt.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", crdKind.Plural(), v1beta2.GroupVersion.Group),
		}, crd,
	); err != nil {
		return nil, err
	}

	return crd, nil
}
