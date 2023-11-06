package testutils

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	ErrStatusModuleStateMismatch  = errors.New("status.modules.state not match")
	ErrContainsUnexpectedModules  = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedModules = errors.New("kyma CR not contains expected modules")
)

func NewTestKyma(name string) *v1beta2.Kyma {
	return NewKymaWithSyncLabel(name, apimetav1.NamespaceDefault, v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalClient)
}

// NewKymaWithSyncLabel use this function to initialize kyma CR with SyncStrategyLocalSecret
// are typically used in e2e test, which expect related access secret provided.
func NewKymaWithSyncLabel(name, namespace, channel, syncStrategy string) *v1beta2.Kyma {
	return builder.NewKymaBuilder().
		WithNamePrefix(name).
		WithNamespace(namespace).
		WithAnnotation(watcher.DomainAnnotation, "example.domain.com").
		WithAnnotation(v1beta2.SyncStrategyAnnotation, syncStrategy).
		WithLabel(v1beta2.InstanceIDLabel, "test-instance").
		WithLabel(v1beta2.SyncLabel, v1beta2.EnableLabelValue).
		WithChannel(channel).
		Build()
}

func SyncKyma(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Name:      kyma.Name,
		Namespace: kyma.Namespace,
	}, kyma)
	// It might happen in some test case, kyma get deleted, if you need to make sure Kyma should exist,
	// write expected condition to check it specifically.
	//nolint:wrapcheck
	return client.IgnoreNotFound(err)
}

func KymaExists(ctx context.Context, clnt client.Client, name, namespace string) error {
	kyma, err := GetKyma(ctx, clnt, name, namespace)
	return CRExists(kyma, err)
}

func KymaDeleted(ctx context.Context,
	kymaName string, kymaNamespace string, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if util.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kyma not deleted: %w", err)
	}
	return nil
}

func DeleteKymaByForceRemovePurgeFinalizer(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	if err := SyncKyma(ctx, clnt, kyma); err != nil {
		return fmt.Errorf("sync kyma %w", err)
	}

	if !kyma.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
			controllerutil.RemoveFinalizer(kyma, v1beta2.PurgeFinalizer)
			if err := clnt.Update(ctx, kyma); err != nil {
				return fmt.Errorf("can't remove purge finalizer %w", err)
			}
		}
	}
	return DeleteCR(ctx, clnt, kyma)
}

func DeleteKyma(ctx context.Context,
	clnt client.Client,
	kyma *v1beta2.Kyma,
) error {
	// Foreground deletion is used to make sure the dependents (manifest CR) get deleted first before Kyma is deleted
	propagation := apimetav1.DeletePropagationForeground
	err := clnt.Delete(ctx, kyma, &client.DeleteOptions{PropagationPolicy: &propagation})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("updating kyma failed %w", err)
	}
	return nil
}

func KymaHasDeletionTimestamp(ctx context.Context,
	clnt client.Client,
	kymaName string,
	kymaNamespace string,
) bool {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return false
	}

	return !kyma.GetDeletionTimestamp().IsZero()
}

func DeleteModule(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma, moduleName string) error {
	manifest, err := GetManifest(ctx, clnt,
		kyma.GetName(), kyma.GetNamespace(), moduleName)
	if util.IsNotFound(err) {
		return nil
	}
	err = client.IgnoreNotFound(clnt.Delete(ctx, manifest))
	if err != nil {
		return fmt.Errorf("module not deleted: %w", err)
	}
	return nil
}

func EnableModule(ctx context.Context,
	clnt client.Client,
	kymaName, kymaNamespace string,
	module v1beta2.Module,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	kyma.Spec.Modules = append(
		kyma.Spec.Modules, module)
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("update kyma: %w", err)
	}
	return nil
}

func DisableModule(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, moduleName string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for i, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			kyma.Spec.Modules = removeModuleWithIndex(kyma.Spec.Modules, i)
			break
		}
	}
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("update kyma: %w", err)
	}
	return nil
}

func removeModuleWithIndex(s []v1beta2.Module, index int) []v1beta2.Module {
	return append(s[:index], s[index+1:]...)
}

func UpdateKymaModuleChannel(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, channel string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for i := range kyma.Spec.Modules {
		kyma.Spec.Modules[i].Channel = channel
	}
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("update kyma: %w", err)
	}
	return nil
}

func UpdateKymaLabel(
	ctx context.Context,
	clnt client.Client,
	kymaName, kymaNamespace,
	labelKey, labelValue string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	kyma.Labels[labelKey] = labelValue
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("update kyma: %w", err)
	}
	return nil
}

func GetKyma(ctx context.Context, clnt client.Client, name, namespace string) (*v1beta2.Kyma, error) {
	kymaInCluster := &v1beta2.Kyma{}
	if namespace == "" {
		namespace = apimetav1.NamespaceDefault
	}
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, kymaInCluster)
	if err != nil {
		return nil, fmt.Errorf("get kyma: %w", err)
	}
	return kymaInCluster, nil
}

func KymaIsInState(ctx context.Context, name, namespace string, clnt client.Client, state shared.State) error {
	return CRIsInState(ctx,
		v1beta2.GroupVersion.Group, v1beta2.GroupVersion.Version, string(v1beta2.KymaKind),
		name, namespace,
		[]string{"status", "state"},
		clnt,
		string(state))
}

func ContainsKymaManagerField(
	ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, managerName string,
) (bool, error) {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return false, err
	}
	if kyma.ManagedFields == nil {
		return false, nil
	}

	for _, v := range kyma.ManagedFields {
		if v.Subresource == "status" && v.Manager == managerName {
			return true, nil
		}
	}

	return false, nil
}

func CheckModuleState(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, moduleName string,
	state shared.State,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	moduleFound := false
	for _, moduleStatus := range kyma.Status.Modules {
		if moduleStatus.Name == moduleName {
			if moduleStatus.State != state {
				return ErrStatusModuleStateMismatch
			}
			moduleFound = true
		}
	}
	if !moduleFound {
		return ErrStatusModuleStateMismatch
	}

	return nil
}

func NotContainsModuleInSpec(ctx context.Context,
	clnt client.Client,
	kymaName, kymaNamespace,
	moduleName string,
) error {
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

func ContainsModuleInSpec(ctx context.Context,
	clnt client.Client,
	kymaName, kymaNamespace,
	moduleName string,
) error {
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
