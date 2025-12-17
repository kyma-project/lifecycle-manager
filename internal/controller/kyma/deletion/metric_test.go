package deletion_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	kymadeletionctrl "github.com/kyma-project/lifecycle-manager/internal/controller/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	"github.com/kyma-project/lifecycle-manager/internal/result/kyma/usecase"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type metricRepoStub struct {
	recordCalled bool

	reasons     []metrics.KymaRequeueReason
	requeueType queue.RequeueType
}

func (m *metricRepoStub) RecordRequeueReason(reason metrics.KymaRequeueReason, requeueType queue.RequeueType) {
	m.recordCalled = true
	if m.reasons == nil {
		m.reasons = []metrics.KymaRequeueReason{}
	}
	m.reasons = append(m.reasons, reason)
	m.requeueType = requeueType
}

func TestMetricWriter_Write(t *testing.T) {
	tests := []struct {
		name                string
		useCase             result.UseCase
		err                 error
		expectedCall        bool
		expectedReasons     []metrics.KymaRequeueReason
		expectedRequeueType queue.RequeueType
	}{
		// Intended requeues
		{
			name:                "StatusUpdateToDeleting intended",
			useCase:             usecase.SetKcpKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.KymaDeletion, metrics.StatusUpdateToDeleting},
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "StatusSyncToRemote intended",
			useCase:             usecase.SetSkrKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.StatusSyncToRemote},
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteKymaDeletion intended",
			useCase:             usecase.DeleteSkrKyma,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteKymaDeletion},
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MT intended",
			useCase:             usecase.DeleteSkrModuleTemplateCrd,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteModuleCatalogDeletion},
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MRM intended",
			useCase:             usecase.DeleteSkrModuleReleaseMetaCrd,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteModuleCatalogDeletion},
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "CleanupManifestCrs intended",
			useCase:             usecase.DeleteManifests,
			err:                 nil,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.CleanupManifestCrs},
			expectedRequeueType: queue.IntendedRequeue,
		},

		// Unexpected requeues
		{
			name:                "StatusUpdateToDeleting unexpected",
			useCase:             usecase.SetKcpKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.KymaDeletion, metrics.StatusUpdateToDeleting},
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "StatusSyncToRemote unexpected",
			useCase:             usecase.SetSkrKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.StatusSyncToRemote},
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteKymaDeletion unexpected",
			useCase:             usecase.DeleteSkrKyma,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteKymaDeletion},
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MT unexpected",
			useCase:             usecase.DeleteSkrModuleTemplateCrd,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteModuleCatalogDeletion},
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MRM unexpected",
			useCase:             usecase.DeleteSkrModuleReleaseMetaCrd,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.RemoteModuleCatalogDeletion},
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "CleanupManifestCrs unexpected",
			useCase:             usecase.DeleteManifests,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReasons:     []metrics.KymaRequeueReason{metrics.CleanupManifestCrs},
			expectedRequeueType: queue.UnexpectedRequeue,
		},

		// No calls
		{
			name:         "Nothing for UseCaseDeleteSkrWatcher",
			useCase:      usecase.DeleteSkrWebhookResources,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for DeleteSkrKymaCrd",
			useCase:      usecase.DeleteSkrKymaCrd,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for DeleteWatcherCertificateSetup",
			useCase:      usecase.DeleteWatcherCertificateSetup,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteMetrics",
			useCase:      usecase.DeleteMetrics,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDropKymaFinalizer",
			useCase:      usecase.DropKymaFinalizer,
			err:          nil,
			expectedCall: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			res := result.Result{
				UseCase: testCase.useCase,
				Err:     testCase.err,
			}

			repoStub := &metricRepoStub{}
			writer := kymadeletionctrl.NewMetricWriter(repoStub)

			writer.Write(res)

			assert.Equal(t, testCase.expectedCall, repoStub.recordCalled)
			if testCase.expectedCall {
				assert.ElementsMatch(t, testCase.expectedReasons, repoStub.reasons)
				assert.Equal(t, testCase.expectedRequeueType, repoStub.requeueType)
			}
		})
	}
}
