package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestExtractOwnerKey(t *testing.T) {
	tests := []struct {
		name        string
		eventObj    *unstructured.Unstructured
		expected    client.ObjectKey
		expectError bool
		errorType   error
	}{
		{
			name: "valid owner extraction",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"owner": map[string]interface{}{
						"name":      "test-kyma",
						"namespace": "kyma-system",
					},
				},
			},
			expected: client.ObjectKey{
				Name:      "test-kyma",
				Namespace: "kyma-system",
			},
			expectError: false,
		},
		{
			name: "missing owner field",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"watched": map[string]interface{}{
						"name": "some-manifest",
					},
				},
			},
			expectError: true,
			errorType:   ErrMissingOwner,
		},
		{
			name: "invalid owner format",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"owner": "invalid-string-format",
				},
			},
			expectError: true,
			errorType:   ErrInvalidOwnerFormat,
		},
		{
			name: "cluster-scoped owner (empty namespace)",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"owner": map[string]interface{}{
						"name":      "cluster-resource",
						"namespace": "",
					},
				},
			},
			expected: client.ObjectKey{
				Name:      "cluster-resource",
				Namespace: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractOwnerKey(tt.eventObj)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExtractWatchedKey(t *testing.T) {
	tests := []struct {
		name        string
		eventObj    *unstructured.Unstructured
		expected    client.ObjectKey
		expectError bool
		errorType   error
	}{
		{
			name: "valid watched extraction",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"watched": map[string]interface{}{
						"name":      "test-manifest",
						"namespace": "default",
					},
				},
			},
			expected: client.ObjectKey{
				Name:      "test-manifest",
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name: "missing watched field",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"owner": map[string]interface{}{
						"name": "some-kyma",
					},
				},
			},
			expectError: true,
			errorType:   ErrMissingWatched,
		},
		{
			name: "invalid watched format",
			eventObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"watched": []string{"invalid", "array", "format"},
				},
			},
			expectError: true,
			errorType:   ErrInvalidWatchedFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractWatchedKey(tt.eventObj)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBothExtractionFunctions(t *testing.T) {
	eventObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"owner": map[string]interface{}{
				"name":      "owner-kyma",
				"namespace": "kyma-system",
			},
			"watched": map[string]interface{}{
				"name":      "watched-manifest",
				"namespace": "default",
			},
		},
	}

	ownerKey, err := ExtractOwnerKey(eventObj)
	require.NoError(t, err)
	assert.Equal(t, "owner-kyma", ownerKey.Name)
	assert.Equal(t, "kyma-system", ownerKey.Namespace)

	watchedKey, err := ExtractWatchedKey(eventObj)
	require.NoError(t, err)
	assert.Equal(t, "watched-manifest", watchedKey.Name)
	assert.Equal(t, "default", watchedKey.Namespace)
}
