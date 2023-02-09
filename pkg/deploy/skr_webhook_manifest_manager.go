package deploy

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
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
	config        *SkrWebhookManagerConfig
	kcpAddr       string
	baseResources []*unstructured.Unstructured
}

func NewSKRWebhookManifestManager(kcpRestConfig *rest.Config, managerConfig *SkrWebhookManagerConfig,
) (*SKRWebhookManifestManager, error) {
	logger := logf.FromContext(context.TODO())
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, managerConfig.SKRWatcherPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, err
	}
	defer closeFileAndLogErr(rawManifestFile, logger, manifestFilePath)
	baseResources, err := getRawManifestUnstructuredResources(rawManifestFile)
	if err != nil {
		return nil, err
	}
	resolvedKcpAddr, err := resolveKcpAddr(kcpRestConfig, managerConfig)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookManifestManager{
		config:        managerConfig,
		kcpAddr:       resolvedKcpAddr,
		baseResources: baseResources,
	}, nil
}

func (m *SKRWebhookManifestManager) Install(ctx context.Context, kyma *v1beta1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	resources, err := m.getSKRClientObjectsForInstall(ctx, syncContext.ControlPlaneClient, kymaObjKey, remoteNs)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(remoteNs)
			return clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
		})
	if err != nil {
		return fmt.Errorf("failed to apply webhook resources: %w", err)
	}
	kyma.UpdateCondition(v1beta1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}

func (m *SKRWebhookManifestManager) Remove(ctx context.Context, kyma *v1beta1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	remoteNs := resolveRemoteNamespace(kyma)
	skrClientObjects := m.getBaseClientObjects()
	genClientObjects := getGeneratedClientObjects(&unstructuredResourcesConfig{}, map[string]WatchableConfig{}, remoteNs)
	skrClientObjects = append(skrClientObjects, genClientObjects...)
	err := runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, skrClientObjects,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(remoteNs)
			return clt.Delete(ctx, resource)
		})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}
	logger.Info("successfully removed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}

func (m *SKRWebhookManifestManager) getSKRClientObjectsForInstall(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey, remoteNs string,
) ([]client.Object, error) {
	var skrClientObjects []client.Object
	resourcesConfig, err := m.getUnstructuredResourcesConfig(ctx, kcpClient, kymaObjKey, remoteNs)
	if err != nil {
		return nil, err
	}
	resources, err := m.getRawManifestClientObjects(resourcesConfig)
	if err != nil {
		return nil, err
	}
	skrClientObjects = append(skrClientObjects, resources...)
	watchableConfigs, err := getWatchableConfigs(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	genClientObjects := getGeneratedClientObjects(resourcesConfig, watchableConfigs, remoteNs)
	return append(skrClientObjects, genClientObjects...), nil
}

func (m *SKRWebhookManifestManager) getRawManifestClientObjects(cfg *unstructuredResourcesConfig,
) ([]client.Object, error) {
	if cfg == nil {
		return nil, ErrExpectedNonNilConfig
	}
	resources := make([]client.Object, 0)
	for _, baseRes := range m.baseResources {
		configuredResource, err := configureUnstructuredObject(cfg, baseRes)
		if err != nil {
			return nil, fmt.Errorf("failed to configure %s resource: %w", baseRes.GetKind(), err)
		}
		resources = append(resources, configuredResource)
	}
	return resources, nil
}

func (m *SKRWebhookManifestManager) getUnstructuredResourcesConfig(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey, remoteNs string,
) (*unstructuredResourcesConfig, error) {
	tlsSecret := &corev1.Secret{}
	secretObjKey := client.ObjectKey{
		Namespace: kymaObjKey.Namespace,
		Name:      ResolveTLSConfigSecretName(kymaObjKey.Name),
	}

	if err := kcpClient.Get(ctx, secretObjKey, tlsSecret); err != nil {
		return nil, fmt.Errorf("error fetching TLS secret: %w", err)
	}

	return &unstructuredResourcesConfig{
		contractVersion:  version,
		kcpAddress:       m.kcpAddr,
		tlsWebhookServer: "true",
		tlsCallback:      "false",
		secretResVer:     tlsSecret.ResourceVersion,
		cpuResLimit:      m.config.SkrWebhookCPULimits,
		memResLimit:      m.config.SkrWebhookMemoryLimits,
		caCert:           tlsSecret.Data[caCertKey],
		tlsCert:          tlsSecret.Data[tlsCertKey],
		tlsKey:           tlsSecret.Data[tlsPrivateKeyKey],
		remoteNs:         remoteNs,
	}, nil
}

func (m *SKRWebhookManifestManager) getBaseClientObjects() []client.Object {
	if m.baseResources == nil || len(m.baseResources) == 0 {
		return nil
	}
	baseClientObjects := make([]client.Object, 0)
	for _, res := range m.baseResources {
		baseClientObjects = append(baseClientObjects, res)
	}
	return baseClientObjects
}
