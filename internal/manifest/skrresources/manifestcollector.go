package skrresources

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/manifestclient"
)

// ManifestLogCollector is a collector for remote Manifest objects.
// It delegates the calls to the embedded generic collector if collection is enabled for the given Manifest.
type ManifestLogCollector struct {
	collector *LogCollector
	enabled   bool
}

func NewManifestLogCollector(manifest *v1beta2.Manifest, owner client.FieldOwner) *ManifestLogCollector {
	key := ""
	enabled := false
	if manifest != nil {
		key = string(manifest.GetUID())
		enabled = isManifestCollectionEnabled(manifest)
	}
	return &ManifestLogCollector{
		collector: NewLogCollector(key, manifestclient.DefaultFieldOwner),
		enabled:   enabled,
	}
}

// Implements the skrresources.ManagedFieldsCollector interface.
func (c *ManifestLogCollector) Collect(ctx context.Context, obj client.Object) {
	if c.enabled {
		c.collector.Collect(ctx, obj)
	}
}

// Implements the skrresources.ManagedFieldsCollector interface.
func (c *ManifestLogCollector) Emit(ctx context.Context) error {
	if c.enabled {
		return c.collector.Emit(ctx)
	}
	return nil
}

// isManifestCollectionEnabled checks if managed fields detection is enabled for the given manifest.
// The detection is disabled by default, but can be enabled by setting a specific label on the manifest CR.
func isManifestCollectionEnabled(obj *v1beta2.Manifest) bool {
	if obj == nil {
		return false
	}

	configuredLabelName := getManagedFieldsAnalysisLabel()

	if configuredLabelName == "" {
		return false
	}

	_, found := obj.GetLabels()[configuredLabelName]
	return found
}
