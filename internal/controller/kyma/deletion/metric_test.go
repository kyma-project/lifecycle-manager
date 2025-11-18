package deletion_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	kymadeletionctrl "github.com/kyma-project/lifecycle-manager/internal/controller/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	resultkymadeletion "github.com/kyma-project/lifecycle-manager/internal/result/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

type metricRepoStub struct {
	recordCalled bool

	reason      metrics.KymaRequeueReason
	requeueType queue.RequeueType
}

func (m *metricRepoStub) RecordRequeueReason(reason metrics.KymaRequeueReason, requeueType queue.RequeueType) {
	m.recordCalled = true
	m.reason = reason
	m.requeueType = requeueType
}

func TestMetricWriter_Write(t *testing.T) {
	tests := []struct {
		name                string
		useCase             result.UseCase
		err                 error
		expectedCall        bool
		expectedReason      metrics.KymaRequeueReason
		expectedRequeueType queue.RequeueType
	}{
		// Intended requeues
		{
			name:                "StatusUpdateToDeleting intended",
			useCase:             resultkymadeletion.UseCaseSetKcpKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "StatusSyncToRemote intended",
			useCase:             resultkymadeletion.UseCaseSetSkrKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteKymaDeletion intended",
			useCase:             resultkymadeletion.UseCaseDeleteSkrKyma,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion intended",
			useCase:             resultkymadeletion.UseCaseDeleteSkrModuleMetadata,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "CleanupManifestCrs intended",
			useCase:             resultkymadeletion.UseCaseDeleteManifests,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
			expectedRequeueType: queue.IntendedRequeue,
		},

		// Unexpected requeues
		{
			name:                "StatusUpdateToDeleting unexpected",
			useCase:             resultkymadeletion.UseCaseSetKcpKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "StatusSyncToRemote unexpected",
			useCase:             resultkymadeletion.UseCaseSetSkrKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteKymaDeletion unexpected",
			useCase:             resultkymadeletion.UseCaseDeleteSkrKyma,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion unexpected",
			useCase:             resultkymadeletion.UseCaseDeleteSkrModuleMetadata,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "CleanupManifestCrs unexpected",
			useCase:             resultkymadeletion.UseCaseDeleteManifests,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
			expectedRequeueType: queue.UnexpectedRequeue,
		},

		// No calls
		{
			name:         "Nothing for UseCaseDeleteSkrWatcher",
			useCase:      resultkymadeletion.UseCaseDeleteSkrWatcher,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteSkrCrds",
			useCase:      resultkymadeletion.UseCaseDeleteSkrCrds,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteWatcherCertificate",
			useCase:      resultkymadeletion.UseCaseDeleteWatcherCertificate,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteMetrics",
			useCase:      resultkymadeletion.UseCaseDeleteMetrics,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseRemoveKymaFinalizers",
			useCase:      resultkymadeletion.UseCaseRemoveKymaFinalizers,
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
				assert.Equal(t, testCase.expectedReason, repoStub.reason)
				assert.Equal(t, testCase.expectedRequeueType, repoStub.requeueType)
			}
		})
	}
}
