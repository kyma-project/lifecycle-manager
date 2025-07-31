package watcher_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

type mockClient struct {
	getFunc  func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
	listFunc func(context.Context, client.ObjectList, ...client.ListOption) error
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return m.getFunc(ctx, key, obj, opts...)
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.listFunc(ctx, list, opts...)
}

func TestAssertDeploymentReady_ReturnsNoError_WhenDeploymentReady(t *testing.T) {
	readyDeployment := &apiappsv1.Deployment{
		Status: apiappsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	getFunc := func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		deployment, _ := obj.(*apiappsv1.Deployment)
		*deployment = *readyDeployment
		return nil
	}
	mockClnt := &mockClient{getFunc: getFunc}
	ctx := t.Context()

	err := watcher.AssertDeploymentReady(ctx, mockClnt)
	require.NoError(t, err)
}

func TestAssertDeploymentReady_ReturnsError_WhenDeploymentNotReady(t *testing.T) {
	notReadyDeployment := &apiappsv1.Deployment{
		Status: apiappsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}
	getFunc := func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		deployment, _ := obj.(*apiappsv1.Deployment)
		*deployment = *notReadyDeployment
		return nil
	}
	mockClnt := &mockClient{getFunc: getFunc}
	ctx := t.Context()

	err := watcher.AssertDeploymentReady(ctx, mockClnt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has no ready replicas")
}

func TestAssertDeploymentReady_ReturnsError_WhenClientReturnsError(t *testing.T) {
	notFoundFunc := func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		return errors.New("deployment not found")
	}
	mockClnt := &mockClient{getFunc: notFoundFunc}
	ctx := t.Context()

	err := watcher.AssertDeploymentReady(ctx, mockClnt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "deployment not found")
}
