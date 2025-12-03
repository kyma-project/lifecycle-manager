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
			useCase:             usecase.SetKcpKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "StatusSyncToRemote intended",
			useCase:             usecase.SetSkrKymaStateDeleting,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteKymaDeletion intended",
			useCase:             usecase.DeleteSkrKyma,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MT intended",
			useCase:             usecase.DeleteSkrModuleTemplateCrd,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MRM intended",
			useCase:             usecase.DeleteSkrModuleReleaseMetaCrd,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.IntendedRequeue,
		},
		{
			name:                "CleanupManifestCrs intended",
			useCase:             usecase.DeleteManifests,
			err:                 nil,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
			expectedRequeueType: queue.IntendedRequeue,
		},

		// Unexpected requeues
		{
			name:                "StatusUpdateToDeleting unexpected",
			useCase:             usecase.SetKcpKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusUpdateToDeleting,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "StatusSyncToRemote unexpected",
			useCase:             usecase.SetSkrKymaStateDeleting,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.StatusSyncToRemote,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteKymaDeletion unexpected",
			useCase:             usecase.DeleteSkrKyma,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteKymaDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MT unexpected",
			useCase:             usecase.DeleteSkrModuleTemplateCrd,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "RemoteModuleCatalogDeletion MRM unexpected",
			useCase:             usecase.DeleteSkrModuleReleaseMetaCrd,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.RemoteModuleCatalogDeletion,
			expectedRequeueType: queue.UnexpectedRequeue,
		},
		{
			name:                "CleanupManifestCrs unexpected",
			useCase:             usecase.DeleteManifests,
			err:                 assert.AnError,
			expectedCall:        true,
			expectedReason:      metrics.CleanupManifestCrs,
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
			name:         "Nothing for UseCaseDeleteWatcherCertificate",
			useCase:      usecase.DeleteWatcherCertificate,
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
			name:         "Nothing for UseCaseRemoveKymaFinalizers",
			useCase:      usecase.RemoveKymaFinalizers,
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
