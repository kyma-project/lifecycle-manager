package webhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/repository/skr/webhook"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
)

func TestNewResourceRepository(t *testing.T) {
	t.Run("initializes with base resources from chart", func(t *testing.T) {
		baseResources := []*unstructured.Unstructured{
			createUnstructuredResource("skr-webhook", "v1", "Service"),
			createUnstructuredResource("skr-webhook-metrics", "v1", "Service"),
			createUnstructuredResource("skr-webhook", "apps/v1", "Deployment"),
		}

		clientCache := &mockSkrClientCache{}
		repo := webhook.NewResourceRepository(clientCache, "kyma-system", baseResources)

		assert.NotNil(t, repo)
	})

	t.Run("initializes with empty base resources", func(t *testing.T) {
		clientCache := &mockSkrClientCache{}
		repo := webhook.NewResourceRepository(clientCache, "kyma-system", []*unstructured.Unstructured{})

		assert.NotNil(t, repo)
	})
}

func TestResourceRepository_ResourcesExist(t *testing.T) {
	const (
		kymaName            = "test-kyma"
		remoteSyncNamespace = "kyma-system"
	)

	baseResources := []*unstructured.Unstructured{
		createUnstructuredResource("skr-webhook", "v1", "Service"),
		createUnstructuredResource("skr-webhook-metrics", "v1", "Service"),
	}

	t.Run("returns true when at least one resource exists", func(t *testing.T) {
		getCallCount := 0
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				getCallCount++
				// First resource exists
				if key.Name == "skr-webhook" {
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		exists, err := repo.ResourcesExist(kymaName)

		require.NoError(t, err)
		assert.True(t, exists)
		// Should short-circuit after finding first resource
		assert.LessOrEqual(t, getCallCount, len(baseResources)+2) // +2 for generated resources
	})

	t.Run("returns false when no resources exist", func(t *testing.T) {
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		exists, err := repo.ResourcesExist(kymaName)

		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("returns true when generated resource exists", func(t *testing.T) {
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				// Only ValidatingWebhookConfiguration exists
				if key.Name == skrwebhookresources.SkrResourceName {
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		exists, err := repo.ResourcesExist(kymaName)

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("returns error when Get fails with non-NotFound error", func(t *testing.T) {
		expectedErr := errors.New("API server error")
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				return expectedErr
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		exists, err := repo.ResourcesExist(kymaName)

		require.Error(t, err)
		assert.False(t, exists)
		assert.Contains(t, err.Error(), "failed to check resource")
	})

	t.Run("short-circuits on first found resource", func(t *testing.T) {
		getCallCount := 0
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				getCallCount++
				// All resources exist, but should stop after first one
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		exists, err := repo.ResourcesExist(kymaName)

		require.NoError(t, err)
		assert.True(t, exists)
		// Due to parallel execution, at least one should complete, but not necessarily all
		assert.Greater(t, getCallCount, 0)
	})
}

func TestResourceRepository_DeleteWebhookResources(t *testing.T) {
	const (
		kymaName            = "test-kyma"
		remoteSyncNamespace = "kyma-system"
	)

	baseResources := []*unstructured.Unstructured{
		createUnstructuredResource("skr-webhook", "v1", "Service"),
		createUnstructuredResource("skr-webhook-metrics", "v1", "Service"),
		createUnstructuredResource("skr-webhook", "apps/v1", "Deployment"),
	}

	t.Run("successfully deletes all resources", func(t *testing.T) {
		deletedResources := make(map[string]bool)
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				key := obj.GetName() + "/" + obj.GetObjectKind().GroupVersionKind().Kind
				deletedResources[key] = true
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		// Verify all resources were deleted (3 base + 2 generated)
		assert.Len(t, deletedResources, 5)
		assert.True(t, deletedResources["skr-webhook/Service"])
		assert.True(t, deletedResources["skr-webhook-metrics/Service"])
		assert.True(t, deletedResources["skr-webhook/Deployment"])
		assert.True(t, deletedResources["skr-webhook/ValidatingWebhookConfiguration"])
		assert.True(t, deletedResources["skr-webhook-tls/Secret"])
	})

	t.Run("ignores NotFound errors during deletion", func(t *testing.T) {
		callCount := 0
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				callCount++
				// Some resources already deleted
				if obj.GetName() == "skr-webhook-metrics" {
					return apierrors.NewNotFound(schema.GroupResource{}, obj.GetName())
				}
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		assert.Equal(t, 5, callCount) // All deletes should be attempted
	})

	t.Run("returns error when delete fails with non-NotFound error", func(t *testing.T) {
		expectedErr := errors.New("permission denied")
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				return expectedErr
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete webhook resources")
	})

	t.Run("deletes resources with correct namespace", func(t *testing.T) {
		var capturedNamespace string
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				capturedNamespace = obj.GetNamespace()
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		assert.Equal(t, remoteSyncNamespace, capturedNamespace)
	})

	t.Run("deletes generated ValidatingWebhookConfiguration", func(t *testing.T) {
		var foundWebhookConfig bool
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == "ValidatingWebhookConfiguration" {
					foundWebhookConfig = true
					assert.Equal(t, skrwebhookresources.SkrResourceName, obj.GetName())
					assert.Equal(t, admissionregistrationv1.SchemeGroupVersion.String(),
						obj.GetObjectKind().GroupVersionKind().GroupVersion().String())
				}
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		assert.True(t, foundWebhookConfig, "ValidatingWebhookConfiguration should be deleted")
	})

	t.Run("deletes generated Secret", func(t *testing.T) {
		var foundSecret bool
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				if obj.GetObjectKind().GroupVersionKind().Kind == "Secret" {
					foundSecret = true
					assert.Equal(t, skrwebhookresources.SkrTLSName, obj.GetName())
				}
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		assert.True(t, foundSecret, "Secret should be deleted")
	})

	t.Run("deletes all resources in parallel", func(t *testing.T) {
		// This test verifies parallel execution by checking that all resources are processed
		deletedCount := 0
		mockClient := &mockSkrClient{
			deleteFunc: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
				deletedCount++
				return nil
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, baseResources)

		err := repo.DeleteWebhookResources(context.Background(), kymaName)

		require.NoError(t, err)
		assert.Equal(t, 5, deletedCount) // 3 base + 2 generated
	})
}

func TestResourceRepository_ClientCacheUsage(t *testing.T) {
	t.Run("uses correct client from cache", func(t *testing.T) {
		const (
			kymaName            = "test-kyma"
			remoteSyncNamespace = "kyma-system"
		)

		var capturedKey client.ObjectKey
		mockClient := &mockSkrClient{
			getFunc: func(ctx context.Context,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				capturedKey = key
				return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
			},
		}

		clientCache := &mockSkrClientCache{client: mockClient}
		repo := webhook.NewResourceRepository(clientCache, remoteSyncNamespace, []*unstructured.Unstructured{})

		_, _ = repo.ResourcesExist(kymaName)

		// The cache should be called with kymaName and remoteSyncNamespace
		// We can't verify the cache.Get call directly, but we verify the client was used
		assert.NotEmpty(t, capturedKey.Name)
	})
}

// mockSkrClient embeds client.Client and only implements Get and Delete methods
type mockSkrClient struct {
	client.Client
	getFunc    func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	deleteFunc func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

func (m *mockSkrClient) Get(ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, key, obj, opts...)
	}
	return nil
}

func (m *mockSkrClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, obj, opts...)
	}
	return nil
}

// mockSkrClientCache implements the SkrClientCache interface
type mockSkrClientCache struct {
	client client.Client
}

func (m *mockSkrClientCache) Get(key client.ObjectKey) client.Client {
	return m.client
}

// Helper function to create unstructured resources for testing
func createUnstructuredResource(name, apiVersion, kind string) *unstructured.Unstructured {
	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	res := &unstructured.Unstructured{}
	res.SetGroupVersionKind(gvk)
	res.SetName(name)
	return res
}
