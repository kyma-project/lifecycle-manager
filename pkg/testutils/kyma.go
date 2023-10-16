package testutils

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var ErrStatusModuleStateMismatch = errors.New("status.modules.state not match")

func NewTestKyma(name string) *v1beta2.Kyma {
	return NewKymaWithSyncLabel(name, v1.NamespaceDefault, v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalClient)
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

func GetKyma(ctx context.Context, clnt client.Client, name, namespace string) (*v1beta2.Kyma, error) {
	kymaInCluster := &v1beta2.Kyma{}
	if namespace == "" {
		namespace = v1.NamespaceDefault
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

func IsKymaInState(ctx context.Context, kcpClient client.Client, kymaName string, state v1beta2.State) bool {
	kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName, "")
	if err != nil || kymaFromCluster.Status.State != state {
		return false
	}
	return true
}

func ExpectKymaManagerField(
	ctx context.Context, controlPlaneClient client.Client, kymaName string, managerName string,
) (bool, error) {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return false, err
	}
	if createdKyma.ManagedFields == nil {
		return false, nil
	}

	for _, v := range createdKyma.ManagedFields {
		if v.Subresource == "status" && v.Manager == managerName {
			return true, nil
		}
	}

	return false, nil
}

func CheckModuleState(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace, moduleName string,
	state v1beta2.State,
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
