package manifest

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

func NewReconciler(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionClient orphan.DetectionRepository,
) *declarativev2.Reconciler {
	kcp := &declarativev2.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}

	// TODO: Create according to flag
	keyChainLookup := keychainprovider.NewDefaultKeyChainProvider()
	statefulChecker := statecheck.NewStatefulSetStateCheck()
	deploymentChecker := statecheck.NewDeploymentStateCheck()
	return declarativev2.NewFromManager(
		mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics, manifestClient, orphanDetectionClient,
		manifest.NewSpecResolver(keyChainLookup, img.NewPathExtractor()),
		declarativev2.WithCustomStateCheck(statecheck.NewManagerStateCheck(statefulChecker, deploymentChecker)),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
	)
}
