package composition

import (
	"path/filepath"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/cmd/composition/service/skrwebhook"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"github.com/kyma-project/lifecycle-manager/internal/repository/istiogateway"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	testskrcontext "github.com/kyma-project/lifecycle-manager/tests/integration/commontestutils/skrcontextimpl"
)

const (
	selfSignedCertDuration    = 1 * time.Hour
	selfSignedCertRenewBefore = 5 * time.Minute
)

func ComposeSkrWebhookManager(
	kcpClient client.Client,
	testSkrContextFactory *testskrcontext.DualClusterFactory,
	gatewayRepository *istiogateway.Repository,
	certificateRepository skrwebhook.CertificateRepository,
	flagVar *flags.FlagVar,
) *watcher.SkrWebhookManifestManager {
	flagVar.SelfSignedCertDuration = selfSignedCertDuration
	flagVar.SelfSignedCertRenewBefore = selfSignedCertRenewBefore
	flagVar.WatcherImageRegistry = "dummyhost"
	flagVar.WatcherImageName = "fake-watcher-image"
	flagVar.WatcherImageTag = "latest"
	flagVar.WatcherResourceLimitsCPU = "200Mi"
	flagVar.WatcherResourceLimitsMemory = "1"

	skrWebhookManager, _ := skrwebhook.ComposeSkrWebhookManager(kcpClient,
		testSkrContextFactory,
		gatewayRepository,
		certificateRepository,
		flagVar,
		filepath.Join(integration.GetProjectRoot(), "skr-webhook"),
	)
	return skrWebhookManager
}
