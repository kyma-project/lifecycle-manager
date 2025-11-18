package deletion_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/internal/controller/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/result"
	resultdeletion "github.com/kyma-project/lifecycle-manager/internal/result/kyma/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/stretchr/testify/assert"
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
			useCase:             resultdeletion.UseCaseSetKcpKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "StatusSyncToRemote intended",
			useCase:             resultdeletion.UseCaseSetSkrKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteKymaDeletion intended",
			useCase:             resultdeletion.UseCaseDeleteSkrKyma,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion intended",
			useCase:             resultdeletion.UseCaseDeleteSkrModuleMetadata,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "CleanupManifestCrs intended",
			useCase:             resultdeletion.UseCaseDeleteManifests,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
			expectedRequeueType: queue.IntendedRequeue,
		},

		// Unexpected requeues
		{
			name:                "StatusUpdateToDeleting unexpected",
			useCase:             resultdeletion.UseCaseSetKcpKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "StatusSyncToRemote unexpected",
			useCase:             resultdeletion.UseCaseSetSkrKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteKymaDeletion unexpected",
			useCase:             resultdeletion.UseCaseDeleteSkrKyma,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion unexpected",
			useCase:             resultdeletion.UseCaseDeleteSkrModuleMetadata,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "CleanupManifestCrs unexpected",
			useCase:             resultdeletion.UseCaseDeleteManifests,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
			expectedRequeueType: queue.UnexpectedRequeue,
		},

		// No calls
		{
			name:         "Nothing for UseCaseDeleteSkrWatcher",
			useCase:      resultdeletion.UseCaseDeleteSkrWatcher,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteSkrCrds",
			useCase:      resultdeletion.UseCaseDeleteSkrCrds,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteWatcherCertificate",
			useCase:      resultdeletion.UseCaseDeleteWatcherCertificate,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseDeleteMetrics",
			useCase:      resultdeletion.UseCaseDeleteMetrics,
			err:          nil,
			expectedCall: false,
		},
		{
			name:         "Nothing for UseCaseRemoveKymaFinalizers",
			useCase:      resultdeletion.UseCaseRemoveKymaFinalizers,
			err:          nil,
			expectedCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := result.Result{
				UseCase: tt.useCase,
				Err:     tt.err,
			}

			repoStub := &metricRepoStub{}
			writer := deletion.NewMetricWriter(repoStub)

			writer.Write(res)

			assert.Equal(t, tt.expectedCall, repoStub.recordCalled)
			if tt.expectedCall {
				assert.Equal(t, tt.expectedReason, repoStub.reason)
				assert.Equal(t, tt.expectedRequeueType, repoStub.requeueType)
			}
		})
	}
}
