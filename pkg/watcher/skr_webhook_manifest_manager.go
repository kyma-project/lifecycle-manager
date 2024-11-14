package watcher

import (
	"context"
	"errors"
	"fmt"
	apicorev1 "k8s.io/api/core/v1"
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SKRWebhookManifestManager struct {
	kcpClient         client.Client
	skrContextFactory remote.SkrContextProvider
	config            SkrWebhookManagerConfig
	kcpAddr           string
	baseResources     []*unstructured.Unstructured
	WatcherMetrics    *metrics.WatcherMetrics
	certificateConfig CertificateConfig
}

type SkrWebhookManagerConfig struct {
	// SKRWatcherPath represents the path of the webhook resources
	// to be installed on SKR clusters upon reconciling kyma CRs.
	SKRWatcherPath         string
	SkrWatcherImage        string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
	RemoteSyncNamespace    string
}

const rawManifestFilePathTpl = "%s/resources.yaml"

func NewSKRWebhookManifestManager(
	kcpClient client.Client,
	skrContextFactory remote.SkrContextProvider,
	managerConfig SkrWebhookManagerConfig,
	certificateConfig CertificateConfig,
	resolvedKcpAddr string,
) (*SKRWebhookManifestManager, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, managerConfig.SKRWatcherPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file path: %w", err)
	}
	defer closeFileAndLogErr(ctx, rawManifestFile, manifestFilePath)
	baseResources, err := getRawManifestUnstructuredResources(rawManifestFile)
	if err != nil {
		return nil, err
	}

	return &SKRWebhookManifestManager{
		kcpClient:         kcpClient,
		skrContextFactory: skrContextFactory,
		config:            managerConfig,
		certificateConfig: certificateConfig,
		kcpAddr:           resolvedKcpAddr,
		baseResources:     baseResources,
		WatcherMetrics:    metrics.NewWatcherMetrics(),
	}, nil
}

func (m *SKRWebhookManifestManager) Install(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	skrContext, err := m.skrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	if _, err = gatewaysecret.GetGatewaySecret(ctx, m.kcpClient); err != nil {
		return err
	}

	// Create CertificateCR which will be used for mTLS connection from SKR to KCP
	certificateMgr := NewCertificateManager(m.kcpClient, kyma.Name,
		m.certificateConfig)

	certificate, err := certificateMgr.CreateSelfSignedCert(ctx, kyma)
	if err != nil {
		return fmt.Errorf("error while patching certificate: %w", err)
	}

	m.updateCertNotRenewMetrics(certificate, kyma)

	if err := certificateMgr.RemoveSecretAfterCARotated(ctx, kymaObjKey); err != nil {
		return fmt.Errorf("error verify CA cert rotation: %w", err)
	}

	logger.V(log.DebugLevel).Info("Successfully created Certificate", "kyma", kymaObjKey)

	resources, err := m.getSKRClientObjectsForInstall(
		ctx, kymaObjKey, m.config.RemoteSyncNamespace, logger)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, skrContext.Client, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(m.config.RemoteSyncNamespace)
			err := clt.Patch(ctx, resource, client.Apply, client.ForceOwnership, skrChartFieldOwner)
			if err != nil {
				return fmt.Errorf("failed to patch resource %s: %w", resource.GetName(), err)
			}
			return nil
		})
	if err != nil {
		return fmt.Errorf("failed to apply webhook resources: %w", err)
	}
	logger.V(log.DebugLevel).Info("successfully installed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}

func (m *SKRWebhookManifestManager) updateCertNotRenewMetrics(certificate *certmanagerv1.Certificate,
	kyma *v1beta2.Kyma,
) {
	if certificate.Status.RenewalTime != nil &&
		time.Now().Add(-m.certificateConfig.RenewBuffer).After(certificate.Status.RenewalTime.Time) {
		m.WatcherMetrics.SetCertNotRenew(kyma.Name)
	} else {
		m.WatcherMetrics.CleanupMetrics(kyma.Name)
	}
}

func (m *SKRWebhookManifestManager) Remove(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	skrContext, err := m.skrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}
	certificate := NewCertificateManager(m.kcpClient, kyma.Name,
		m.certificateConfig)
	if err = certificate.Remove(ctx); err != nil {
		return err
	}
	skrClientObjects := m.getBaseClientObjects()
	genClientObjects := getGeneratedClientObjects(&unstructuredResourcesConfig{}, []v1beta2.Watcher{},
		m.config.RemoteSyncNamespace)
	skrClientObjects = append(skrClientObjects, genClientObjects...)
	err = runResourceOperationWithGroupedErrors(ctx, skrContext.Client, skrClientObjects,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(m.config.RemoteSyncNamespace)
			err = clt.Delete(ctx, resource)
			if err != nil {
				return fmt.Errorf("failed to delete resource %s: %w", resource.GetName(), err)
			}
			return nil
		})
	if err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}
	logger.V(log.DebugLevel).Info("successfully removed webhook resources",
		"kyma", kymaObjKey.String())
	return nil
}

func (m *SKRWebhookManifestManager) getSKRClientObjectsForInstall(ctx context.Context,
	kymaObjKey client.ObjectKey, remoteNs string, logger logr.Logger,
) ([]client.Object, error) {
	var skrClientObjects []client.Object
	resourcesConfig, err := m.getUnstructuredResourcesConfig(ctx, kymaObjKey, remoteNs)
	if err != nil {
		return nil, err
	}
	resources, err := m.getRawManifestClientObjects(resourcesConfig)
	if err != nil {
		return nil, err
	}
	skrClientObjects = append(skrClientObjects, resources...)
	watchers, err := getWatchers(ctx, m.kcpClient)
	if err != nil {
		return nil, err
	}
	logger.V(log.DebugLevel).Info(fmt.Sprintf("using %d watchers to generate webhook configs", len(watchers)))
	genClientObjects := getGeneratedClientObjects(resourcesConfig, watchers, remoteNs)
	return append(skrClientObjects, genClientObjects...), nil
}

var errExpectedNonNilConfig = errors.New("expected non nil config")

func (m *SKRWebhookManifestManager) getRawManifestClientObjects(cfg *unstructuredResourcesConfig,
) ([]client.Object, error) {
	if cfg == nil {
		return nil, errExpectedNonNilConfig
	}
	resources := make([]client.Object, 0)
	for _, baseRes := range m.baseResources {
		resource := baseRes.DeepCopy()
		resource.SetLabels(collections.MergeMaps(resource.GetLabels(), map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		}))
		configuredResource, err := configureUnstructuredObject(cfg, resource)
		if err != nil {
			return nil, fmt.Errorf("failed to configure %s resource: %w", resource.GetKind(), err)
		}
		resources = append(resources, configuredResource)
	}
	return resources, nil
}

func (m *SKRWebhookManifestManager) getUnstructuredResourcesConfig(ctx context.Context,
	kymaObjKey client.ObjectKey, remoteNs string,
) (*unstructuredResourcesConfig, error) {
	tlsSecret := &apicorev1.Secret{}
	certObjKey := client.ObjectKey{
		Namespace: m.certificateConfig.IstioNamespace,
		Name:      ResolveTLSCertName(kymaObjKey.Name),
	}

	if err := m.kcpClient.Get(ctx, certObjKey, tlsSecret); err != nil {
		if util.IsNotFound(err) {
			return nil, &CertificateNotReadyError{}
		}
		return nil, fmt.Errorf("error fetching TLS secret: %w", err)
	}

	gatewaySecret, err := gatewaysecret.GetGatewaySecret(ctx, m.kcpClient)
	if err != nil {
		return nil, fmt.Errorf("error fetching gateway secret: %w", err)
	}

	return &unstructuredResourcesConfig{
		contractVersion: version,
		kcpAddress:      m.kcpAddr,
		secretResVer:    tlsSecret.ResourceVersion,
		cpuResLimit:     m.config.SkrWebhookCPULimits,
		memResLimit:     m.config.SkrWebhookMemoryLimits,
		skrWatcherImage: m.config.SkrWatcherImage,
		caCert:          gatewaySecret.Data[caCertKey],
		tlsCert:         tlsSecret.Data[tlsCertKey],
		tlsKey:          tlsSecret.Data[tlsPrivateKeyKey],
		remoteNs:        remoteNs,
	}, nil
}

func (m *SKRWebhookManifestManager) getBaseClientObjects() []client.Object {
	if len(m.baseResources) == 0 {
		return nil
	}
	baseClientObjects := make([]client.Object, 0)
	for _, res := range m.baseResources {
		resCopy := res.DeepCopy()
		baseClientObjects = append(baseClientObjects, resCopy)
	}
	return baseClientObjects
}
