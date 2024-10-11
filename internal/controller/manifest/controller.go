package manifest

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=manifests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=manifests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=manifests/finalizers,verbs=update

func NewReconciler(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient declarativev2.ManifestAPIClient,
) *declarativev2.Reconciler {
	kcp := &declarativev2.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	extractor := img.NewPathExtractor()
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}
	keyChainLookup := manifest.NewKeyChainProvider(kcp.Client)
	statefulChecker := statecheck.NewStatefulSetStateCheck()
	deploymentChecker := statecheck.NewDeploymentStateCheck()
	return declarativev2.NewFromManager(
		mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics, manifestClient,
		manifest.NewSpecResolver(keyChainLookup, extractor),
		declarativev2.WithCustomStateCheck(statecheck.NewManagerStateCheck(statefulChecker, deploymentChecker)),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
		declarativev2.WithPostRun{manifest.PostRunCreateCR},
		declarativev2.WithPreDelete{manifest.PreDeleteDeleteCR},
	)
}
