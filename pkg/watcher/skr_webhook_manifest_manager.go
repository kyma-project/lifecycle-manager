package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

// WatcherManifestManager is a SKRWebhookManager implementation that applies
// the SKR webhook's raw manifest using a native kube-client.
type WatcherManifestManager struct {
	config             WatcherManagerConfig
	kcpAddr            string
	baseResources      []*unstructured.Unstructured
	caCertificateCache *CACertificateCache
	WatcherMetrics     *metrics.WatcherMetrics
	certificateConfig  CertificateConfig
}

type WatcherManagerConfig struct {
	// WatcherResourcesPath represents the path of the webhook resources
	// to be installed on SKR clusters upon reconciling kyma CRs.
	WatcherResourcesPath string
	// WatcherImage contains the full registry URL of the runtime-watcher image to be used on the remote cluster.
	WatcherImage        string
	WatcherMemoryLimits string
	WatcherCpuLimits    string
	// RemoteSyncNamespace indicates the sync namespace for Kyma and module catalog
	RemoteSyncNamespace string
}

func NewWatcherManifestManager(kcpConfig *rest.Config,
	schema *machineryruntime.Scheme,
	caCertificateCache *CACertificateCache,
	managerConfig WatcherManagerConfig,
	certificateConfig CertificateConfig,
	gatewayConfig GatewayConfig,
) (*WatcherManifestManager, error) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	manifestFilePath := fmt.Sprintf("%s/resources.yaml", managerConfig.WatcherResourcesPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file path: %w", err)
	}
	defer closeFileAndLogErr(ctx, rawManifestFile, manifestFilePath)
	baseResources, err := getRawManifestUnstructuredResources(rawManifestFile)
	if err != nil {
		return nil, err
	}
	kcpClient, err := client.New(kcpConfig, client.Options{Scheme: schema})
	if err != nil {
		return nil, fmt.Errorf("can't create kcpClient: %w", err)
	}
	resolvedKcpAddr, err := resolveKcpAddr(ctx, kcpClient, gatewayConfig)
	if err != nil {
		return nil, err
	}
	return &WatcherManifestManager{
		config:             managerConfig,
		certificateConfig:  certificateConfig,
		kcpAddr:            resolvedKcpAddr,
		baseResources:      baseResources,
		caCertificateCache: caCertificateCache,
		WatcherMetrics:     metrics.NewWatcherMetrics(),
	}, nil
}

func (m *WatcherManifestManager) Install(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}

	// Create CertificateCR which will be used for mTLS connection from SKR to KCP
	certificateMgr := NewCertificateManager(syncContext.ControlPlaneClient, kyma.Name,
		m.certificateConfig, m.caCertificateCache)

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
		ctx, syncContext.ControlPlaneClient, kymaObjKey, m.config.RemoteSyncNamespace, logger)
	if err != nil {
		return err
	}
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, resources,
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

func (m *WatcherManifestManager) updateCertNotRenewMetrics(certificate *certmanagerv1.Certificate,
	kyma *v1beta2.Kyma,
) {
	if certificate.Status.RenewalTime != nil &&
		time.Now().Add(-m.certificateConfig.RenewBuffer).After(certificate.Status.RenewalTime.Time) {
		m.WatcherMetrics.SetCertNotRenew(kyma.Name)
	} else {
		m.WatcherMetrics.CleanupMetrics(kyma.Name)
	}
}

func (m *WatcherManifestManager) Remove(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}
	certificate := NewCertificateManager(syncContext.ControlPlaneClient, kyma.Name,
		m.certificateConfig, m.caCertificateCache)
	if err = certificate.Remove(ctx); err != nil {
		return err
	}
	skrClientObjects := m.getBaseClientObjects()
	genClientObjects := getGeneratedClientObjects(&unstructuredResourcesConfig{}, []v1beta2.Watcher{},
		m.config.RemoteSyncNamespace)
	skrClientObjects = append(skrClientObjects, genClientObjects...)
	err = runResourceOperationWithGroupedErrors(ctx, syncContext.RuntimeClient, skrClientObjects,
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

func (m *WatcherManifestManager) getSKRClientObjectsForInstall(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey, remoteNs string, logger logr.Logger,
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
	watchers, err := getWatchers(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	logger.V(log.DebugLevel).Info(fmt.Sprintf("using %d watchers to generate webhook configs", len(watchers)))
	genClientObjects := getGeneratedClientObjects(resourcesConfig, watchers, remoteNs)
	return append(skrClientObjects, genClientObjects...), nil
}

var errExpectedNonNilConfig = errors.New("expected non nil config")

func (m *WatcherManifestManager) getRawManifestClientObjects(cfg *unstructuredResourcesConfig,
) ([]client.Object, error) {
	if cfg == nil {
		return nil, errExpectedNonNilConfig
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

func (m *WatcherManifestManager) getUnstructuredResourcesConfig(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey, remoteNs string,
) (*unstructuredResourcesConfig, error) {
	tlsSecret := &apicorev1.Secret{}
	certObjKey := client.ObjectKey{
		Namespace: m.certificateConfig.IstioNamespace,
		Name:      ResolveTLSCertName(kymaObjKey.Name),
	}

	if err := kcpClient.Get(ctx, certObjKey, tlsSecret); err != nil {
		if util.IsNotFound(err) {
			return nil, &CertificateNotReadyError{}
		}
		return nil, fmt.Errorf("error fetching TLS secret: %w", err)
	}

	return &unstructuredResourcesConfig{
		contractVersion: version,
		kcpAddress:      m.kcpAddr,
		secretResVer:    tlsSecret.ResourceVersion,
		cpuResLimit:     m.config.WatcherCpuLimits,
		memResLimit:     m.config.WatcherMemoryLimits,
		skrWatcherImage: m.config.WatcherImage,
		caCert:          tlsSecret.Data[caCertKey],
		tlsCert:         tlsSecret.Data[tlsCertKey],
		tlsKey:          tlsSecret.Data[tlsPrivateKeyKey],
		remoteNs:        remoteNs,
	}, nil
}

func (m *WatcherManifestManager) getBaseClientObjects() []client.Object {
	if m.baseResources == nil || len(m.baseResources) == 0 {
		return nil
	}
	baseClientObjects := make([]client.Object, 0)
	for _, res := range m.baseResources {
		baseClientObjects = append(baseClientObjects, res)
	}
	return baseClientObjects
}
