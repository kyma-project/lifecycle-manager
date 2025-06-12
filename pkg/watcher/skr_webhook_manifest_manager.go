package watcher

import (
	"context"
	"errors"
	"fmt"

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
	"github.com/kyma-project/lifecycle-manager/internal/service/skrwebhook/chartreader"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/pkg/watcher/skr_webhook_resources"
)

var ErrSkrCertificateNotReady = errors.New("SKR certificate not ready")

const (
	skrChartFieldOwner = client.FieldOwner(shared.OperatorName)
)

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
	GetSkrCertificateSecretData(ctx context.Context, kymaName string) (*secret.CertificateSecretData, error)
	GetGatewayCertificateSecretData(ctx context.Context) (*secret.GatewaySecretData, error)
}

type SkrWebhookManifestManager struct {
	kcpClient            client.Client
	skrContextFactory    remote.SkrContextProvider
	remoteSyncNamespace  string
	kcpAddr              skrwebhookresources.KCPAddr
	chartReaderService   *chartreader.Service
	baseResources        []*unstructured.Unstructured
	watcherMetrics       WatcherMetrics
	certificateManager   CertificateManager
	resourceConfigurator *skrwebhookresources.ResourceConfigurator
}

func NewSKRWebhookManifestManager(kcpClient client.Client, skrContextFactory remote.SkrContextProvider,
	remoteSyncNamespace string, resolvedKcpAddr skrwebhookresources.KCPAddr, chartReaderService *chartreader.Service,
	certificateManager CertificateManager, resourceConfigurator *skrwebhookresources.ResourceConfigurator,
	watcherMetrics *metrics.WatcherMetrics,
) (*SkrWebhookManifestManager, error) {
	baseResources, err := chartReaderService.GetRawManifestUnstructuredResources()
	if err != nil {
		return nil, err
	}
	return &SkrWebhookManifestManager{
		kcpClient:            kcpClient,
		skrContextFactory:    skrContextFactory,
		remoteSyncNamespace:  remoteSyncNamespace,
		kcpAddr:              resolvedKcpAddr,
		chartReaderService:   chartReaderService,
		baseResources:        baseResources,
		watcherMetrics:       watcherMetrics,
		certificateManager:   certificateManager,
		resourceConfigurator: resourceConfigurator,
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
		ctx, kyma.Name, logger)
	if err != nil {
		return err
	}
	err = m.chartReaderService.RunResourceOperationWithGroupedErrors(ctx, skrContext.Client, resources,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(m.remoteSyncNamespace)
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
	genClientObjects := m.getGeneratedClientObjects(secret.CertificateSecretData{}, secret.GatewaySecretData{},
		[]v1beta2.Watcher{})
	skrClientObjects = append(skrClientObjects, genClientObjects...)
	err = m.chartReaderService.RunResourceOperationWithGroupedErrors(ctx, skrContext.Client, skrClientObjects,
		func(ctx context.Context, clt client.Client, resource client.Object) error {
			resource.SetNamespace(m.remoteSyncNamespace)
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

func (m *SkrWebhookManifestManager) writeCertificateRenewalMetrics(ctx context.Context, kymaName string,
	logger logr.Logger,
) {
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
	logger logr.Logger,
) ([]client.Object, error) {
	var skrClientObjects []client.Object
	resources, err := m.getRawManifestClientObjects(ctx, kymaName)
	if err != nil {
		return nil, err
	}
	skrClientObjects = append(skrClientObjects, resources...)
	watchers, err := getWatchers(ctx, m.kcpClient)
	if err != nil {
		return nil, err
	}
	logger.V(log.DebugLevel).Info(fmt.Sprintf("using %d watchers to generate webhook configs", len(watchers)))
	skrCertificateSecretData, gatewaySecretData, err := m.getCertificateData(ctx, kymaName)
	if err != nil {
		return nil, err
	}
	genClientObjects := m.getGeneratedClientObjects(*skrCertificateSecretData, *gatewaySecretData, watchers)
	return append(skrClientObjects, genClientObjects...), nil
}

func (m *SkrWebhookManifestManager) getGeneratedClientObjects(skrCertificateSecretData secret.CertificateSecretData,
	gatewaySecretData secret.GatewaySecretData,
	watchers []v1beta2.Watcher,
) []client.Object {
	var genClientObjects []client.Object

	webhookConfig := skrwebhookresources.BuildValidatingWebhookConfigFromWatchers(gatewaySecretData.CaCert, watchers,
		m.remoteSyncNamespace)
	genClientObjects = append(genClientObjects, webhookConfig)

	skrSecret := skrwebhookresources.BuildSKRSecret(gatewaySecretData.CaCert, skrCertificateSecretData.TlsCert,
		skrCertificateSecretData.TlsKey, m.remoteSyncNamespace)
	return append(genClientObjects, skrSecret)
}

func (m *SkrWebhookManifestManager) getRawManifestClientObjects(ctx context.Context, kymaName string,
) ([]client.Object, error) {
	resources := make([]client.Object, 0)
	skrCertificateSecret, err := m.certificateManager.GetSkrCertificateSecret(ctx, kymaName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, ErrSkrCertificateNotReady
		}
		return nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	for _, baseRes := range m.baseResources {
		resource := baseRes.DeepCopy()
		resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
			shared.ManagedBy: shared.ManagedByLabelValue,
		}))
		m.resourceConfigurator.SetSecretResVer(skrCertificateSecret.ResourceVersion)
		configuredResource, err := m.resourceConfigurator.ConfigureUnstructuredObject(resource)
		if err != nil {
			return nil, fmt.Errorf("failed to configure %s resource: %w", resource.GetKind(), err)
		}
		resources = append(resources, configuredResource)
	}
	return resources, nil
}

func (m *SkrWebhookManifestManager) getCertificateData(ctx context.Context,
	kymaName string,
) (*secret.CertificateSecretData, *secret.GatewaySecretData, error) {
	skrCertificateSecretData, err := m.certificateManager.GetSkrCertificateSecretData(ctx, kymaName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, ErrSkrCertificateNotReady
		}

		return nil, nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	gatewaySecretData, err := m.certificateManager.GetGatewayCertificateSecretData(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	return skrCertificateSecretData, gatewaySecretData, nil
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

func getWatchers(ctx context.Context, kcpClient client.Client) ([]v1beta2.Watcher, error) {
	watcherList := &v1beta2.WatcherList{}
	if err := kcpClient.List(ctx, watcherList); err != nil {
		return nil, fmt.Errorf("error listing watcher CRs: %w", err)
	}

	return watcherList.Items, nil
}
