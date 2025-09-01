package manifest

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/statecheck"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/orphan"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

func NewReconciler(mgr manager.Manager,
	requeueIntervals queue.RequeueIntervals,
	manifestMetrics *metrics.ManifestMetrics,
	mandatoryModulesMetrics *metrics.MandatoryModulesMetrics,
	manifestClient declarativev2.ManifestAPIClient,
	orphanDetectionClient orphan.DetectionRepository,
	specResolver *spec.Resolver,
	clientCache declarativev2.SKRClientCache,
	clientFactory declarativev2.SKRClientFactory,
	accessManagerService *accessmanager.Service,
) *declarativev2.Reconciler {
	kcp := &skrclient.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	lookup := &manifest.NewRemoteClusterLookup(kcp, accessManagerService)
	statefulChecker := statecheck.NewStatefulSetStateCheck()
	deploymentChecker := statecheck.NewDeploymentStateCheck()
	return declarativev2.NewFromManager(
		mgr, requeueIntervals, manifestMetrics, mandatoryModulesMetrics, manifestClient, orphanDetectionClient,
		specResolver,
		clientCache,
		clientFactory,
		declarativev2.WithCustomStateCheck(statecheck.NewManagerStateCheck(statefulChecker, deploymentChecker)),
		declarativev2.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
	)
}
