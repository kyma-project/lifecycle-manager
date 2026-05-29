package resources_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func Test_DeleteDiffResourcesWhenManifestUnderDeleting(t *testing.T) {
	tests := []struct {
		name                string
		clientError         error
		expectManifestState shared.State
	}{
		{
			"Given resources deletion not finished, expect manifest CR state to Warning",
			resources.ErrDeletionNotFinished,
			shared.StateWarning,
		},
		{
			"Given resources deletion with unexpect error, expect manifest CR state to Error",
			errors.New("unexpected error"),
			shared.StateError,
		},
		{
			"Given resources deletion with not found error, expect manifest CR state not change",
			apierrors.NewNotFound(schema.GroupResource{}, "manifest not found"),
			shared.StateDeleting,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			objs := []shared.Resource{
				makeResource("sa", "default", "ServiceAccount"),
				makeResource("deploy", "default", "Deployment"),
				makeResource("sample", "default", "Sample"),
				makeResource("managed", "default", "Managed"),
			}
			fakeClient := NewErrorMockFakeClient(testCase.clientError)
			manifest := testutils.NewTestManifest("test")
			manifest.Status.State = shared.StateDeleting
			cleanup := resources.NewConcurrentCleanup(fakeClient, manifest)
			_ = cleanup.DeleteDiffResources(t.Context(), objs)
			if manifest.Status.State != testCase.expectManifestState {
				t.Errorf("DeleteDiffResources() manifest.Status.State = %v, want %v",
					manifest.Status.State, testCase.expectManifestState)
			}
		})
	}
}

type ErrorMockFakeClient struct {
	client.WithWatch

	failError error
}

func NewErrorMockFakeClient(failError error) *ErrorMockFakeClient {
	builder := fake.NewClientBuilder()
	return &ErrorMockFakeClient{
		WithWatch: builder.Build(),
		failError: failError,
	}
}

func (c *ErrorMockFakeClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	if c.failError != nil {
		return c.failError
	}

	return nil
}

func Test_IsOperatorRelatedResources(t *testing.T) {
	tests := []struct {
		name  string
		kinds []string
		want  bool
	}{
		{
			"operator related resources should be determined",
			[]string{
				"Namespace", "ServiceAccount", "Service",
				"Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding",
				"Deployment", "CustomResourceDefinition",
			},
			true,
		},
		{
			"non operator related resources should be ignored",
			[]string{"Pod"},
			false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for _, kind := range testCase.kinds {
				if got := resources.IsOperatorRelatedResources(kind); got != testCase.want {
					t.Errorf("IsOperatorRelatedResources(%q) = %v, want %v", kind, got, testCase.want)
				}
			}
		})
	}
}

func Test_SplitResources(t *testing.T) {
	tests := []struct {
		name                     string
		resources                []shared.Resource
		operatorRelatedResources []shared.Resource
		operatorManagedResources []shared.Resource
	}{
		{
			"resources split correctly",
			[]shared.Resource{
				makeResource("ns", "", "Namespace"),
				makeResource("crd", "", "CustomResourceDefinition"),
				makeResource("sa", "default", "ServiceAccount"),
				makeResource("deploy", "default", "Deployment"),
				makeResource("sample", "default", "Sample"),
				makeResource("managed", "default", "Managed"),
			},
			[]shared.Resource{
				makeResource("ns", "", "Namespace"),
				makeResource("crd", "", "CustomResourceDefinition"),
				makeResource("sa", "default", "ServiceAccount"),
				makeResource("deploy", "default", "Deployment"),
			},
			[]shared.Resource{
				makeResource("sample", "default", "Sample"),
				makeResource("managed", "default", "Managed"),
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualRelated, actualManaged := resources.SplitResources(testCase.resources)

			if !reflect.DeepEqual(actualRelated, testCase.operatorRelatedResources) {
				t.Errorf("SplitResources() operatorRelatedResources = %v, want %v",
					actualRelated, testCase.operatorRelatedResources)
			}
			if !reflect.DeepEqual(actualManaged, testCase.operatorManagedResources) {
				t.Errorf("SplitResources() operatorManagedResources = %v, want %v",
					actualManaged, testCase.operatorManagedResources)
			}
		})
	}
}

func makeResource(name, namespace, kind string) shared.Resource {
	return shared.Resource{
		Name:      name,
		Namespace: namespace,
		GroupVersionKind: apimetav1.GroupVersionKind{
			Kind: kind,
		},
	}
}
