package testutils

import (
	"context"
	"errors"
	"fmt"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrStatusModuleStateMismatch            = errors.New("status.modules.state not match")
	ErrContainsUnexpectedModules            = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedModules           = errors.New("kyma CR not contains expected modules")
	ErrModuleVersionInStatusIsIncorrect     = errors.New("status.modules.version is incorrect")
	ErrModuleMaintenanceInStatusIsIncorrect = errors.New("status.modules.maintenance is incorrect")
	ErrModuleMessageInStatusIsIncorrect     = errors.New("status.modules.message is incorrect")
)

const (
	FastChannel = "fast"
)

func NewTestKyma(name string) *v1beta2.Kyma {
	return NewKymaWithNamespaceName(name, ControlPlaneNamespace, v1beta2.DefaultChannel)
}

func NewSKRKyma() *v1beta2.Kyma {
	return builder.NewKymaBuilder().
		WithName(shared.DefaultRemoteKymaName).
		WithNamespace(shared.DefaultRemoteNamespace).
		WithChannel(v1beta2.DefaultChannel).
		Build()
}

// NewKymaWithNamespaceName use this function to initialize kyma CR with SyncStrategyLocalSecret
// are typically used in e2e test, which expect related access secret provided.
func NewKymaWithNamespaceName(name, namespace, channel string) *v1beta2.Kyma {
	return builder.NewKymaBuilder().
		WithNamePrefix(name).
		WithNamespace(namespace).
		WithAnnotation(shared.SkrDomainAnnotation, "example.domain.com").
		WithLabel(shared.InstanceIDLabel, "test-instance").
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
	err = client.IgnoreNotFound(err)
	if err != nil {
		return fmt.Errorf("failed to fetch Kyma CR: %w", err)
	}
	return nil
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
		if controllerutil.ContainsFinalizer(kyma, shared.PurgeFinalizer) {
			controllerutil.RemoveFinalizer(kyma, shared.PurgeFinalizer)
			if err := clnt.Update(ctx, kyma); err != nil {
				return fmt.Errorf("can't remove purge finalizer %w", err)
			}
		}
	}
	return DeleteKyma(ctx, clnt, kyma, apimetav1.DeletePropagationBackground)
}

func DeleteKyma(ctx context.Context,
	clnt client.Client,
	kyma *v1beta2.Kyma,
	deletionPropagation apimetav1.DeletionPropagation,
) error {
	err := clnt.Delete(ctx, kyma, &client.DeleteOptions{PropagationPolicy: &deletionPropagation})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("updating kyma failed %w", err)
	}
	return nil
}

// UpdateKymaWithFunc uses the provided function to update the Kyma resource.
// This function is intended to be used with "Eventually" assertions in tests.
// The provided updateFn should modify the Kyma resource in place and return an error if the modification fails.
// UpdateKymaWithFunc always fetches the latest version of the Kyma resource before applying changes
// to make sure the update is based on the most recent state.
func UpdateKymaWithFunc(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace string, updateFn func(kyma *v1beta2.Kyma) error,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return fmt.Errorf("UpdateKymaWithFunc GetKyma: %w", err)
	}
	err = updateFn(kyma)
	if err != nil {
		return err
	}
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("UpdateKymaWithFunc client.Update: %w", err)
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
	for _, enabledModule := range kyma.Spec.Modules {
		if enabledModule.Name == module.Name {
			return nil
		}
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

func SetModuleManaged(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, moduleName string, managed bool,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	for i, module := range kyma.Spec.Modules {
		if module.Name == moduleName {
			kyma.Spec.Modules[i].Managed = managed
			break
		}
	}
	err = clnt.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("update kyma failed: %w", err)
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

// ImmediatelyRequeueKyma adds a dummy label to the Kyma CR to trigger a requeue.
func ImmediatelyRequeueKyma(
	ctx context.Context,
	clnt client.Client,
	kymaName, kymaNamespace string,
) error {
	return UpdateKymaLabel(ctx, clnt, kymaName, kymaNamespace, "operator.kyma-project.io/dummy-label", random.Name())
}

func GetKyma(ctx context.Context, clnt client.Client, name, namespace string) (*v1beta2.Kyma, error) {
	kymaInCluster := &v1beta2.Kyma{}

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
		v1beta2.GroupVersion.Group, v1beta2.GroupVersion.Version, string(shared.KymaKind),
		name, namespace,
		[]string{"status", "state"},
		clnt,
		state)
}

func SetKymaState(ctx context.Context, kyma *v1beta2.Kyma, clnt client.Client, state shared.State) error {
	kyma.Status = v1beta2.KymaStatus{
		State:         state,
		Conditions:    nil,
		Modules:       nil,
		ActiveChannel: "",
		LastOperation: shared.LastOperation{LastUpdateTime: apimetav1.NewTime(time.Now())},
	}
	kyma.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1beta2.GroupVersion.Group,
		Version: v1beta2.GroupVersion.Version,
		Kind:    string(shared.KymaKind),
	})
	kyma.ManagedFields = nil

	return clnt.Status().Patch(ctx, kyma, client.Apply,
		status.SubResourceOpts(client.ForceOwnership),
		client.FieldOwner(shared.OperatorName))
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

func ModuleMessageInKymaStatusIsCorrect(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, moduleName, message string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	for _, module := range kyma.Status.Modules {
		if module.Name == moduleName && module.Message == message {
			return nil
		}
	}

	return ErrModuleMessageInStatusIsIncorrect
}

func ModuleVersionInKymaStatusIsCorrect(ctx context.Context,
	clnt client.Client, kymaName, kymaNamespace, moduleName, moduleVersion string,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	for _, module := range kyma.Status.Modules {
		if module.Name == moduleName && module.Version == moduleVersion {
			return nil
		}
	}

	return ErrModuleVersionInStatusIsIncorrect
}

func ModuleMaintenanceIndicatorInKymaStatusIsCorrect(ctx context.Context,
	clnt client.Client, kymaName, kymaNamespace, moduleName string, underMaintenanceWindow bool,
) error {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return err
	}

	for _, module := range kyma.Status.Modules {
		if module.Name == moduleName && module.Maintenance == underMaintenanceWindow {
			return nil
		}
	}

	return ErrModuleMaintenanceInStatusIsIncorrect
}

// AddManifestToKymaStatus adds a reference of the provided module in the status.modules in the Kyma CR
// to prevent Manifest reconciliation error due to orphaned module.
func AddManifestToKymaStatus(ctx context.Context, kcpClient client.Client,
	kymaName, kymaNamespace, manifestName string,
) error {
	kyma, err := GetKyma(ctx, kcpClient, kymaName, kymaNamespace)
	if err != nil {
		return err
	}
	kyma.Status.Modules = append(kyma.Status.Modules, v1beta2.ModuleStatus{
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name: manifestName,
			},
		},
	})
	err = kcpClient.Status().Update(ctx, kyma)
	if err != nil {
		return err
	}
	return nil
}
