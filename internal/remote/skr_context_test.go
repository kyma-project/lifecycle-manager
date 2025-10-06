package remote //nolint:testpackage // testing package internals

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

const (
	kymaName      = "test-name"
	kymaNamespace = "test-namespace"
)

func TestReplaceWithVirtualKyma(t *testing.T) {
	t.Parallel()
	type testKyma struct {
		channel     string
		moduleNames []string
	}
	tests := []struct {
		name         string
		remoteKyma   testKyma
		kcpKyma      testKyma
		expectedKyma testKyma
	}{
		{
			"modules in kcp Kyma get replaced with modules in remote Kyma",
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module1", "module3"},
			},
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module2"},
			},
			testKyma{
				channel:     "regular",
				moduleNames: []string{"module1", "module3"},
			},
		},
		{
			"channel in kcp Kym gets replaced with channel in remote Kyma",
			testKyma{
				channel: "regular",
			},
			testKyma{
				channel: "fast",
			},
			testKyma{
				channel: "regular",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			kcpKyma := createKyma(testCase.kcpKyma.channel, testCase.kcpKyma.moduleNames)
			remoteKyma := createKyma(testCase.remoteKyma.channel, testCase.remoteKyma.moduleNames)
			ReplaceSpec(kcpKyma, remoteKyma)
			assert.Equal(t, testCase.expectedKyma.channel, kcpKyma.Spec.Channel)
			var virtualModules []string
			for _, module := range kcpKyma.Spec.Modules {
				virtualModules = append(virtualModules, module.Name)
			}

			require.ElementsMatch(t, testCase.expectedKyma.moduleNames, virtualModules)
		})
	}
}

func Test_SynchronizeKymaMetadata_SkipsIfSKRKymaIsDeleting(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().WithDeletionTimestamp().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.False(t, clientStub.called)
	assert.False(t, eventStub.called)
}

func Test_SynchronizeKymaMetadata_Syncs(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.False(t, eventStub.called)
}

func Test_SynchronizeKymaMetadata_SkipsSyncIfLabelsAndAnnotationsUnchanged(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, kymaNamespace, kymaName)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.False(t, clientStub.called)
	assert.False(t, eventStub.called)
}

func Test_SynchronizeKymaMetadata_ErrorsWhenFailedToSync(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	expectedError := errors.New("test error")
	eventStub := &eventStub{}
	clientStub := &clientStub{err: expectedError}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.ErrorIs(t, err, expectedError)
	assert.Contains(t, err.Error(), "failed to synchronise Kyma metadata to SKR")
	assert.True(t, clientStub.called)
	assert.True(t, eventStub.called)
}

func Test_SynchronizeKymaMetadata_SyncsBTPRelatedLabels(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.False(t, eventStub.called)
	assert.Equal(t, "global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
}

func Test_SynchronizeKymaMetadata_UpdatesBTPRelatedLabels(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "old-global-account-id").
		WithLabel(shared.SubAccountIDLabel, "old-sub-account-id").
		Build()
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "new-global-account-id").
		WithLabel(shared.SubAccountIDLabel, "new-sub-account-id").
		Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.True(t, clientStub.called)
	assert.False(t, eventStub.called)
	assert.Equal(t, "new-global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "new-sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
}

func Test_SynchronizeKymaMetadata_SkipsSyncIfBTPRelatedLabelsUnchanged(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		Build()
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		Build()

	eventStub := &eventStub{}
	clientStub := &clientStub{}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.Equal(t, "global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
}

func Test_SynchronizeKymaMetadata_ErrorsWhenFailedToSyncBTPRelatedLabels(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		Build()

	expectedError := errors.New("test error")
	eventStub := &eventStub{}
	clientStub := &clientStub{err: expectedError}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaMetadata(t.Context(), kcpKyma, skrKyma)

	require.ErrorIs(t, err, expectedError)
	assert.Contains(t, err.Error(), "failed to synchronise Kyma metadata to SKR")
	assert.True(t, clientStub.called)
	assert.True(t, eventStub.called)
}

func Test_SynchronizeKymaStatus_SkipsIfSKRKymaIsDeleting(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().WithDeletionTimestamp().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	eventStub := &eventStub{}
	statusClient := &statusClient{}
	clientStub := &clientStub{status: statusClient}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaStatus(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.False(t, statusClient.called)
	assert.False(t, eventStub.called)
}

func Test_SynchronizeKymaStatus_Syncs(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	eventStub := &eventStub{}
	statusClient := &statusClient{}
	clientStub := &clientStub{status: statusClient}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaStatus(t.Context(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.True(t, statusClient.called)
	assert.False(t, eventStub.called)
}

func Test_SynchronizeKymaStatus_ErrorsWhenFailedToSync(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	expectedError := errors.New("test error")
	eventStub := &eventStub{}
	statusClient := &statusClient{err: expectedError}
	clientStub := &clientStub{status: statusClient}
	skrContext := NewSkrContext(clientStub, eventStub)

	err := skrContext.SynchronizeKymaStatus(t.Context(), kcpKyma, skrKyma)

	require.ErrorIs(t, err, expectedError)
	assert.Contains(t, err.Error(), "failed to synchronise Kyma status to SKR")
	assert.True(t, statusClient.called)
	assert.True(t, eventStub.called)
}

func Test_syncStatus_AssignsRemoteNamespace(t *testing.T) {
	skrStatus := &v1beta2.KymaStatus{}
	kcpStatus := &v1beta2.KymaStatus{
		Modules: []v1beta2.ModuleStatus{
			{
				Name: "module-1",
				Template: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-2",
				Template: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-3",
			},
		},
	}

	syncStatus(kcpStatus, skrStatus)

	for _, module := range skrStatus.Modules {
		if module.Template == nil {
			continue
		}
		assert.Equal(t, shared.DefaultRemoteNamespace, module.Template.Namespace)
	}

	for _, module := range kcpStatus.Modules {
		if module.Template == nil {
			continue
		}
		assert.Equal(t, "kcp-system", module.Template.Namespace)
	}
}

func Test_syncStatus_RemovesManifestReference(t *testing.T) {
	skrStatus := &v1beta2.KymaStatus{}
	kcpStatus := &v1beta2.KymaStatus{
		Modules: []v1beta2.ModuleStatus{
			{
				Name: "module-1",
				Manifest: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-2",
				Manifest: &v1beta2.TrackingObject{
					PartialMeta: v1beta2.PartialMeta{
						Namespace: "kcp-system",
					},
				},
			},
			{
				Name: "module-3",
			},
		},
	}

	syncStatus(kcpStatus, skrStatus)

	for _, module := range skrStatus.Modules {
		assert.Nil(t, module.Manifest)
	}

	assert.NotNil(t, kcpStatus.Modules[0].Manifest)
	assert.NotNil(t, kcpStatus.Modules[1].Manifest)
	assert.Nil(t, kcpStatus.Modules[2].Manifest)
}

func Test_syncWatcherLabelsAnnotations_AddsLabelsAndAnnotations(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncWatcherLabelsAnnotations_AddsLabels(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, kymaNamespace, kymaName)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncWatcherLabelsAnnotations_AddsAnnotations(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncWatcherLabelsAnnotations_ChangesAnnotations(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, "foo").
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncWatcherLabelsAnnotations_ChangesLabels(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, "foo").
		WithLabel(shared.ManagedBy, "bar").
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, kymaNamespace, kymaName)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncWatcherLabelsAnnotations_ReturnsFalseIfLabelsAndAnnotationsUnchanged(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, kymaNamespace, kymaName)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(kymaName).WithNamespace(kymaNamespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.False(t, changed)
	assertLabelsAndAnnotations(t, skrKyma)
}

func Test_syncBTPRelatedLabels_AddsLabels(t *testing.T) {
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		Build()
	skrKyma := builder.NewKymaBuilder().Build()

	changed := syncBTPRelatedLabels(kcpKyma, skrKyma)

	assert.True(t, changed)
	assert.Equal(t, "global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
}

func Test_syncBTPRelatedLabels_UpdatesLabels(t *testing.T) {
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "new-global-account-id").
		WithLabel(shared.SubAccountIDLabel, "new-sub-account-id").
		Build()
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "old-global-account-id").
		WithLabel(shared.SubAccountIDLabel, "old-sub-account-id").
		Build()

	changed := syncBTPRelatedLabels(kcpKyma, skrKyma)

	assert.True(t, changed)
	assert.Equal(t, "new-global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "new-sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
}

func Test_syncBTPRelatedLabels_NoChange(t *testing.T) {
	kcpKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").Build()
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.GlobalAccountIDLabel, "global-account-id").
		WithLabel(shared.SubAccountIDLabel, "sub-account-id").
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		Build()

	changed := syncBTPRelatedLabels(kcpKyma, skrKyma)

	assert.False(t, changed)
	assert.Equal(t, "global-account-id", skrKyma.Labels[shared.GlobalAccountIDLabel])
	assert.Equal(t, "sub-account-id", skrKyma.Labels[shared.SubAccountIDLabel])
	assert.Equal(t, shared.WatchedByLabelValue, skrKyma.Labels[shared.WatchedByLabel])
	assert.Equal(t, shared.ManagedByLabelValue, skrKyma.Labels[shared.ManagedBy])
}

// test helpers

func createKyma(channel string, moduleNames []string) *v1beta2.Kyma {
	kyma := builder.NewKymaBuilder().
		WithChannel(channel).
		Build()

	modules := make([]v1beta2.Module, 0, len(moduleNames))
	for _, moduleName := range moduleNames {
		modules = append(modules, v1beta2.Module{
			Name:    moduleName,
			Channel: v1beta2.DefaultChannel,
			Managed: true,
		})
	}

	kyma.Spec.Modules = modules

	return kyma
}

func assertLabelsAndAnnotations(t *testing.T, skrKyma *v1beta2.Kyma) {
	t.Helper()

	assert.Equal(t, shared.WatchedByLabelValue, skrKyma.Labels[shared.WatchedByLabel])
	assert.Equal(t, shared.ManagedByLabelValue, skrKyma.Labels[shared.ManagedBy])
	assert.Equal(t,
		fmt.Sprintf(shared.OwnedByFormat, kymaNamespace, kymaName),
		skrKyma.Annotations[shared.OwnedByAnnotation])
}

// test stubs

type eventStub struct {
	event.Event

	called bool
}

func (e *eventStub) Warning(_ machineryruntime.Object, _ event.Reason, _ error) {
	e.called = true
}

type clientStub struct {
	client.Client

	err    error
	status client.SubResourceWriter
	called bool
}

func (c *clientStub) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
	c.called = true
	return c.err
}

func (c *clientStub) Status() client.SubResourceWriter {
	return c.status
}

func (*clientStub) Config() *rest.Config {
	return nil
}

type statusClient struct {
	client.SubResourceWriter

	err    error
	called bool
}

func (s *statusClient) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	s.called = true
	return s.err
}
