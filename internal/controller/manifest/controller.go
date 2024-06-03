package manifest

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func NewReconciler(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
) *declarativev2.Reconciler {
	kcp := &declarativev2.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	extractor := manifest.NewPathExtractor(nil)
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}
	return declarativev2.NewFromManager(
		mgr,
		&v1beta2.Manifest{},
		requeueIntervals,
		manifestMetrics,
		mandatoryModulesMetrics,
		declarativev2.NewInMemoryCachedManifestParser(),
		manifest.NewCustomResourceReadyCheck(),
		declarativev2.NewMemoryClientCache(),
		// Options
		declarativev2.WithSpecResolver(
			manifest.NewSpecResolver(kcp, extractor),
		),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
	)
}
