package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrSkrCertificateNotReady = errors.New("SKR certificate not ready")

type WatcherMetrics interface {
	SetCertNotRenew(kymaName string)
	CleanupMetrics(kymaName string)
}

type CertificateManager interface {
	CreateSkrCertificate(ctx context.Context, kyma *v1beta2.Kyma) error
	RenewSkrCertificate(ctx context.Context, kymaName string) error
	IsSkrCertificateRenewalOverdue(ctx context.Context, kymaName string) (bool, error)
	DeleteSkrCertificate(ctx context.Context, kymaName string) error
	GetSkrCertificateSecret(ctx context.Context, kymaName string) (*apicorev1.Secret, error)
	GetGatewayCertificateSecret(ctx context.Context) (*apicorev1.Secret, error)
}

type KCPAddr struct {
	Hostname string
	Port     int
}

type SkrWebhookManifestManager struct {
	kcpClient          client.Client
	skrContextFactory  remote.SkrContextProvider
	config             SkrWebhookManagerConfig
	kcpAddr            KCPAddr
	baseResources      []*unstructured.Unstructured
	watcherMetrics     WatcherMetrics
	certificateManager CertificateManager
}

type SkrWebhookManagerConfig struct {
	// SkrWatcherPath represents the path of the webhook resources
	// to be installed on SKR clusters upon reconciling kyma CRs.
	SkrWatcherPath         string
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
	resolvedKcpAddr KCPAddr,
	certificateManager CertificateManager,
	watcherMetrics *metrics.WatcherMetrics,
) (*SkrWebhookManifestManager, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, managerConfig.SkrWatcherPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file path: %w", err)
	}
	defer closeFileAndLogErr(ctx, rawManifestFile, manifestFilePath)
	baseResources, err := getRawManifestUnstructuredResources(rawManifestFile)
	if err != nil {
		return nil, err
	}

	return &SkrWebhookManifestManager{
		kcpClient:          kcpClient,
		skrContextFactory:  skrContextFactory,
		config:             managerConfig,
		kcpAddr:            resolvedKcpAddr,
		baseResources:      baseResources,
		watcherMetrics:     watcherMetrics,
		certificateManager: certificateManager,
	}, nil
}

// Reconcile installs and updates the resources of the watch mechanism.
// E.g., it creates, updates and renews the SKR certificate and syncs it to the SKR cluster along
// with the other watcher-related resources like the Deployment and ValidatingWebhookConfiguration.
func (m *SkrWebhookManifestManager) Reconcile(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	skrContext, err := m.skrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	err = m.certificateManager.CreateSkrCertificate(ctx, kyma)
	if err != nil {
		return fmt.Errorf("failed to create SKR certificate: %w", err)
	}

	m.writeCertificateRenewalMetrics(ctx, kyma.Name, logger)

	if err = m.certificateManager.RenewSkrCertificate(ctx, kyma.Name); err != nil {
		return fmt.Errorf("failed to renew SKR certificate: %w", err)
	}

	logger.V(log.DebugLevel).Info("Successfully created Certificate", "kyma", kymaObjKey)

	resources, err := m.getSKRClientObjectsForInstall(
		ctx, kyma.Name, m.config.RemoteSyncNamespace, logger)
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

// Remove removes all resources of the watch mechanism.
func (m *SkrWebhookManifestManager) Remove(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	skrContext, err := m.skrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return fmt.Errorf("failed to get skrContext: %w", err)
	}

	if err = m.certificateManager.DeleteSkrCertificate(ctx, kyma.Name); err != nil {
		return fmt.Errorf("failed to delete SKR certificate: %w", err)
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

	m.watcherMetrics.CleanupMetrics(kyma.Name)

	return nil
}

// RemoveSkrCertificate removes the SKR certificate from the KCP cluster.
// The major anticipated use case is to cleanup orphaned certificates.
func (m *SkrWebhookManifestManager) RemoveSkrCertificate(ctx context.Context, kymaName string) error {
	if err := m.certificateManager.DeleteSkrCertificate(ctx, kymaName); err != nil {
		return fmt.Errorf("failed to delete SKR certificate: %w", err)
	}

	return nil
}

func (m *SkrWebhookManifestManager) writeCertificateRenewalMetrics(ctx context.Context, kymaName string, logger logr.Logger) {
	overdue, err := m.certificateManager.IsSkrCertificateRenewalOverdue(ctx, kymaName)
	if err != nil {
		m.watcherMetrics.SetCertNotRenew(kymaName)
		logger.Error(err, "failed to check if certificate renewal is overdue for kyma "+kymaName)
		return
	}

	if overdue {
		m.watcherMetrics.SetCertNotRenew(kymaName)
		return
	}

	m.watcherMetrics.CleanupMetrics(kymaName)
}

func (m *SkrWebhookManifestManager) getSKRClientObjectsForInstall(ctx context.Context,
	kymaName string,
	remoteNs string,
	logger logr.Logger,
) ([]client.Object, error) {
	var skrClientObjects []client.Object
	resourcesConfig, err := m.getUnstructuredResourcesConfig(ctx, kymaName, remoteNs)
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

func (m *SkrWebhookManifestManager) getRawManifestClientObjects(cfg *unstructuredResourcesConfig,
) ([]client.Object, error) {
	if cfg == nil {
		return nil, errExpectedNonNilConfig
	}
	resources := make([]client.Object, 0)
	for _, baseRes := range m.baseResources {
		resource := baseRes.DeepCopy()
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
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

func (m *SkrWebhookManifestManager) getUnstructuredResourcesConfig(ctx context.Context,
	kymaName string,
	remoteNs string,
) (*unstructuredResourcesConfig, error) {
	skrCertificateSecret, err := m.certificateManager.GetSkrCertificateSecret(ctx, kymaName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrSkrCertificateNotReady
		}

		return nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	gatewaySecret, err := m.certificateManager.GetGatewayCertificateSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	return &unstructuredResourcesConfig{
		contractVersion: version,
		kcpAddress:      m.kcpAddr,
		secretResVer:    skrCertificateSecret.ResourceVersion,
		cpuResLimit:     m.config.SkrWebhookCPULimits,
		memResLimit:     m.config.SkrWebhookMemoryLimits,
		skrWatcherImage: m.config.SkrWatcherImage,
		caCert:          gatewaySecret.Data[caCertKey],
		tlsCert:         skrCertificateSecret.Data[tlsCertKey],
		tlsKey:          skrCertificateSecret.Data[tlsPrivateKeyKey],
		remoteNs:        remoteNs,
	}, nil
}

func (m *SkrWebhookManifestManager) getBaseClientObjects() []client.Object {
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
