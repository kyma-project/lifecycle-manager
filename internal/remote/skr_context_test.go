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

func Test_SynchronizeKyma_SkipsIfSKRKymaIsDeleting(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().WithDeletionTimestamp().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	event := &eventStub{}
	client := &clientStub{}
	skrContext := NewSkrContext(client, event)

	err := skrContext.SynchronizeKyma(context.Background(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.False(t, client.called)
	assert.False(t, event.called)
}

func Test_SynchronizeKyma_Syncs(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	event := &eventStub{}
	statusClient := &statusClient{}
	client := &clientStub{status: statusClient}
	skrContext := NewSkrContext(client, event)

	err := skrContext.SynchronizeKyma(context.Background(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.True(t, client.called)
	assert.True(t, statusClient.called)
	assert.False(t, event.called)
}

func Test_SynchronizeKyma_SkipsSyncIfLabelsAndAnnotationsUnchanged(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"
	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, namespace, name)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	event := &eventStub{}
	statusClient := &statusClient{}
	client := &clientStub{status: statusClient}
	skrContext := NewSkrContext(client, event)

	err := skrContext.SynchronizeKyma(context.Background(), kcpKyma, skrKyma)

	require.NoError(t, err)
	assert.False(t, client.called)
	assert.True(t, statusClient.called)
	assert.False(t, event.called)
}

func Test_SynchronizeKyma_ErrorsWhenFailedToSync(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	expectedError := errors.New("test error")
	event := &eventStub{}
	statusClient := &statusClient{}
	client := &clientStub{err: expectedError, status: statusClient}
	skrContext := NewSkrContext(client, event)

	err := skrContext.SynchronizeKyma(context.Background(), kcpKyma, skrKyma)

	require.ErrorIs(t, err, expectedError)
	assert.Contains(t, err.Error(), "failed to synchronise Kyma to SKR")
	assert.True(t, client.called)
	assert.True(t, event.called)
	assert.False(t, statusClient.called)
}

func Test_SynchronizeKyma_ErrorsWhenFailedToSyncStatus(t *testing.T) {
	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().Build()

	expectedError := errors.New("test error")
	event := &eventStub{}
	statusClient := &statusClient{err: expectedError}
	client := &clientStub{status: statusClient}
	skrContext := NewSkrContext(client, event)

	err := skrContext.SynchronizeKyma(context.Background(), kcpKyma, skrKyma)

	require.ErrorIs(t, err, expectedError)
	assert.Contains(t, err.Error(), "failed to synchronise Kyma status to SKR")
	assert.True(t, client.called)
	assert.True(t, statusClient.called)
	assert.True(t, event.called)
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
		if module.Manifest == nil {
			continue
		}
		assert.Nil(t, module.Manifest)
	}

	assert.NotNil(t, kcpStatus.Modules[0].Manifest)
	assert.NotNil(t, kcpStatus.Modules[1].Manifest)
}

func Test_syncWatcherLabelsAnnotations_AddsLabelsAndAnnotations(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

func Test_syncWatcherLabelsAnnotations_AddsLabels(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, namespace, name)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

func Test_syncWatcherLabelsAnnotations_AddsAnnotations(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

func Test_syncWatcherLabelsAnnotations_ChangesAnnotations(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, "foo").
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

func Test_syncWatcherLabelsAnnotations_ChangesLabels(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, "foo").
		WithLabel(shared.ManagedBy, "bar").
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, namespace, name)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.True(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

func Test_syncWatcherLabelsAnnotations_ReturnsFalseIfLabelsAndAnnotationsUnchanged(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"

	skrKyma := builder.NewKymaBuilder().
		WithLabel(shared.WatchedByLabel, shared.WatchedByLabelValue).
		WithLabel(shared.ManagedBy, shared.ManagedByLabelValue).
		WithAnnotation(shared.OwnedByAnnotation, fmt.Sprintf(shared.OwnedByFormat, namespace, name)).
		Build()
	kcpKyma := builder.NewKymaBuilder().WithName(name).WithNamespace(namespace).Build()

	changed := syncWatcherLabelsAnnotations(kcpKyma, skrKyma)

	assert.False(t, changed)
	assertLabelsAndAnnotations(t, skrKyma, name, namespace)
}

// test helpers

func createKyma(channel string, moduleNames []string) *v1beta2.Kyma {
	kyma := builder.NewKymaBuilder().
		WithChannel(channel).
		Build()

	modules := []v1beta2.Module{}
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

func assertLabelsAndAnnotations(t *testing.T, skrKyma *v1beta2.Kyma, kcpKymaName, kcpKymaNamespace string) {
	t.Helper()

	assert.Equal(t, shared.WatchedByLabelValue, skrKyma.Labels[shared.WatchedByLabel])
	assert.Equal(t, shared.ManagedByLabelValue, skrKyma.Labels[shared.ManagedBy])
	assert.Equal(t,
		fmt.Sprintf(shared.OwnedByFormat, kcpKymaNamespace, kcpKymaName),
		skrKyma.Annotations[shared.OwnedByAnnotation])
}

// test stubs

type eventStub struct {
	event.Event
	called bool
}

func (e *eventStub) Warning(object machineryruntime.Object, reason event.Reason, err error) {
	e.called = true
}

type clientStub struct {
	err    error
	status client.SubResourceWriter
	client.Client
	called bool
}

func (c *clientStub) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
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
	err error
	client.SubResourceWriter
	called bool
}

func (s *statusClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	s.called = true
	return s.err
}
