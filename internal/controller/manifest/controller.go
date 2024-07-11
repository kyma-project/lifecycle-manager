package manifest

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
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
		mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics,
		manifest.NewSpecResolver(kcp.Client, extractor),
		manifest.NewDeploymentReadyCheck(),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
	)
}
