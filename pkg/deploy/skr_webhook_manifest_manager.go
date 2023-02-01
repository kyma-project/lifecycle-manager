package deploy

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"go.uber.org/zap"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// SKRWebhookManifestManager is a SKRWebhookManager implementation that applies
// the SKR webhook's raw manifest using a native kube-client.
type SKRWebhookManifestManager struct {
	config  *SkrChartManagerConfig
	kcpAddr string
}

func NewSKRWebhookManifestManager(kcpRestConfig *rest.Config, managerConfig *SkrChartManagerConfig,
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

func (m *SKRWebhookManifestManager) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	resources, err := getSKRClientObjectsForInstall(ctx, syncContext.ControlPlaneClient, kymaObjKey,
		remoteNs, m.kcpAddr, m.config.WebhookChartPath, logger)
	if err != nil {
		return true, err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			return clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
		})
	if err != nil {
		return true, fmt.Errorf("failed to apply webhook resources: %w", err)
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", skrChartReleaseName(kymaObjKey))
	return false, nil
}

func (m *SKRWebhookManifestManager) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	manifestFilePath := fmt.Sprintf("%s/raw/skr-webhook-resources.yaml", m.config.WebhookChartPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return err
	}
	defer func(closer io.Closer) {
		err := closer.Close()
		if err != nil {
			logger.V(int(zap.DebugLevel)).Info("failed to close raw manifest file", "path",
				manifestFilePath)
		}
	}(rawManifestFile)
	resources, err := getRawManifestUnstructuredResources(rawManifestFile, remoteNs)
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
	logger.Info("successfully removed webhook chart",
		"release-name", skrChartReleaseName(kymaObjKey))
	return nil
}
