package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

// SKRWebhookManifestManager is a SKRWebhookManager implementation that applies
// the SKR webhook's raw manifest using a native kube-client.
type SKRWebhookManifestManager struct {
	config        *SkrWebhookManagerConfig
	kcpAddr       string
	baseResources []*unstructured.Unstructured
}

type SkrWebhookManagerConfig struct {
	// SKRWatcherPath represents the path of the webhook resources
	// to be installed on SKR clusters upon reconciling kyma CRs.
	SKRWatcherPath         string
	SkrWatcherImage        string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
	// IstioNamespace represents the cluster resource namespace of istio
	IstioNamespace string
	// IstioGatewayName represents the cluster resource name of the klm istio gateway
	IstioGatewayName string
	// IstioGatewayNamespace represents the cluster resource namespace of the klm istio gateway
	IstioGatewayNamespace string
	// RemoteSyncNamespace indicates the sync namespace for Kyma and module catalog
	RemoteSyncNamespace string
	// LocalGatewayPortOverwrite indicates the port used to expose the KCP cluster locally in k3d
	// for the watcher callbacks
	LocalGatewayPortOverwrite string
	// AdditionalDNSNames indicates the DNS Names which should be added additional to the Subject
	// Alternative Names of each Kyma Certificate
	AdditionalDNSNames []string
}

const rawManifestFilePathTpl = "%s/resources.yaml"

func NewSKRWebhookManifestManager(kcpConfig *rest.Config,
	schema *runtime.Scheme,
	managerConfig *SkrWebhookManagerConfig,
) (SKRWebhookManager, error) {
	logger := logf.FromContext(context.TODO())
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, managerConfig.SKRWatcherPath)
	rawManifestFile, err := os.Open(manifestFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file path: %w", err)
	}
	defer closeFileAndLogErr(rawManifestFile, logger, manifestFilePath)
	baseResources, err := getRawManifestUnstructuredResources(rawManifestFile)
	if err != nil {
		return nil, err
	}
	kcpClient, err := client.New(kcpConfig, client.Options{Scheme: schema})
	if err != nil {
		return nil, fmt.Errorf("can't create kcpClient: %w", err)
	}
	resolvedKcpAddr, err := resolveKcpAddr(kcpClient, managerConfig)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookManifestManager{
		config:        managerConfig,
		kcpAddr:       resolvedKcpAddr,
		baseResources: baseResources,
	}, nil
}

func (m *SKRWebhookManifestManager) Install(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}

	// Create CertificateCR which will be used for mTLS connection from SKR to KCP
	certificate, err := NewCertificateManager(syncContext.ControlPlaneClient, kyma,
		m.config.IstioNamespace, m.config.RemoteSyncNamespace, m.config.AdditionalDNSNames)
	if err != nil {
		return fmt.Errorf("error while creating new CertificateManager struct: %w", err)
	}
	if err = certificate.Create(ctx); err != nil {
		return fmt.Errorf("error while creating new Certificate on KCP: %w", err)
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

func (m *SKRWebhookManifestManager) Remove(ctx context.Context, kyma *v1beta2.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext, err := remote.SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}
	certificate, err := NewCertificateManager(syncContext.ControlPlaneClient, kyma,
		m.config.IstioNamespace, m.config.RemoteSyncNamespace, []string{})
	if err != nil {
		logger.Error(err, "Error while creating new CertificateManager")
		return err
	}
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

func (m *SKRWebhookManifestManager) getSKRClientObjectsForInstall(ctx context.Context, kcpClient client.Client,
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

func (m *SKRWebhookManifestManager) getRawManifestClientObjects(cfg *unstructuredResourcesConfig,
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

func (m *SKRWebhookManifestManager) getUnstructuredResourcesConfig(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey, remoteNs string,
) (*unstructuredResourcesConfig, error) {
	tlsSecret := &corev1.Secret{}
	certObjKey := client.ObjectKey{
		Namespace: m.config.IstioNamespace,
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
		cpuResLimit:     m.config.SkrWebhookCPULimits,
		memResLimit:     m.config.SkrWebhookMemoryLimits,
		skrWatcherImage: m.config.SkrWatcherImage,
		caCert:          tlsSecret.Data[caCertKey],
		tlsCert:         tlsSecret.Data[tlsCertKey],
		tlsKey:          tlsSecret.Data[tlsPrivateKeyKey],
		remoteNs:        remoteNs,
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
