package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SKRWebhookManifestManager is a SKRWebhookManager implementation that applies
// the SKR webhook's raw manifest using a native kube-client.
type SKRWebhookManifestManager struct {
	config  *SkrWebhookManagerConfig
	kcpAddr string
}

func NewSKRWebhookManifestManager(kcpRestConfig *rest.Config, managerConfig *SkrWebhookManagerConfig,
) (*SKRWebhookManifestManager, error) {
	resolvedKcpAddr, err := resolveKcpAddr(kcpRestConfig, managerConfig)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookManifestManager{
		config:  managerConfig,
		kcpAddr: resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookManifestManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	resources, err := getSKRClientObjectsForInstall(ctx, syncContext.ControlPlaneClient, kymaObjKey,
		remoteNs, m.kcpAddr, logger, m.config)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			return clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
		})
	if err != nil {
		return fmt.Errorf("failed to apply webhook resources: %w", err)
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}

func (m *SKRWebhookManifestManager) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, m.config.WebhookChartPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return err
	}
	defer closeFileAndLogErr(rawManifestFile, logger, manifestFilePath)
	resources, err := getRawManifestUnstructuredResources(rawManifestFile, remoteNs)
	genClientObjects := getGeneratedClientObjects(kymaObjKey, remoteNs,
		&unstructuredResourcesConfig{}, map[string]WatchableConfig{})
	resources = append(resources, genClientObjects...)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			return clt.Delete(ctx, resource)
		})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}
	logger.Info("successfully removed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}
