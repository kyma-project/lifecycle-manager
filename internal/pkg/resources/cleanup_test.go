package resources_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apirbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
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
			resourcesInfo := convertToResourceInfo([]machineryruntime.Object{
				&apicorev1.ServiceAccount{},
				&apiappsv1.Deployment{},
				&templatev1alpha1.Sample{},
				&templatev1alpha1.Managed{},
			})
			fakeClient := NewErrorMockFakeClient(testCase.clientError)
			manifest := testutils.NewTestManifest("test")
			manifest.Status.State = shared.StateDeleting
			cleanup := resources.NewConcurrentCleanup(fakeClient, manifest)
			_ = cleanup.DeleteDiffResources(t.Context(), resourcesInfo)
			if manifest.Status.State != testCase.expectManifestState {
				t.Errorf("SplitResources() manifest.Status.State = %v, want %v",
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
		kinds []any
		want  bool
	}{
		{
			"operator related resources should be determined",
			[]any{
				apicorev1.Namespace{},
				apicorev1.ServiceAccount{},
				apicorev1.Service{},
				apirbacv1.Role{},
				apirbacv1.ClusterRole{},
				apirbacv1.RoleBinding{},
				apirbacv1.ClusterRoleBinding{},
				apiappsv1.Deployment{},
				apiextensionsv1.CustomResourceDefinition{},
			},
			true,
		},
		{
			"non operator related resources should be ignored",
			[]any{
				apicorev1.Pod{},
			},
			false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			for _, kind := range testCase.kinds {
				if got := resources.IsOperatorRelatedResources(getKindName(kind)); got != testCase.want {
					t.Errorf("IsOperatorRelatedResources() = %v, want %v", got, testCase.want)
				}
			}
		})
	}
}

func getKindName(cr any) string {
	t := reflect.TypeOf(cr)
	if t.Kind() == reflect.Ptr {
		return t.Elem().Name()
	} else {
		return t.Name()
	}
}

func Test_SplitResources(t *testing.T) {
	tests := []struct {
		name                     string
		resources                []machineryruntime.Object
		operatorRelatedResources []machineryruntime.Object
		operatorManagedResources []machineryruntime.Object
	}{
		{
			"resources split correctly",
			[]machineryruntime.Object{
				&apicorev1.Namespace{},
				&apiextensionsv1.CustomResourceDefinition{},
				&apicorev1.ServiceAccount{},
				&apiappsv1.Deployment{},
				&templatev1alpha1.Sample{},
				&templatev1alpha1.Managed{},
			},
			[]machineryruntime.Object{
				&apicorev1.Namespace{},
				&apiextensionsv1.CustomResourceDefinition{},
				&apicorev1.ServiceAccount{},
				&apiappsv1.Deployment{},
			},
			[]machineryruntime.Object{
				&templatev1alpha1.Sample{}, &templatev1alpha1.Managed{},
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			expectedOperatorRelatedResources := convertToResourceInfo(testCase.operatorRelatedResources)
			expectedOperatorManagedResources := convertToResourceInfo(testCase.operatorManagedResources)
			resourcesInfo := convertToResourceInfo(testCase.resources)
			actualOperatorRelatedResources, actualOperatorManagedResources, _ := resources.SplitResources(resourcesInfo)

			if !reflect.DeepEqual(actualOperatorRelatedResources, expectedOperatorRelatedResources) {
				t.Errorf("SplitResources() actualOperatorRelatedResources = %v, want %v",
					actualOperatorRelatedResources, testCase.operatorRelatedResources)
			}
			if !reflect.DeepEqual(actualOperatorManagedResources, expectedOperatorManagedResources) {
				t.Errorf("SplitResources() actualOperatorManagedResources = %v, want %v",
					actualOperatorManagedResources, testCase.operatorManagedResources)
			}
		})
	}
}

func convertToResourceInfo(objects []machineryruntime.Object) []*resource.Info {
	items := make([]*resource.Info, 0)
	for _, object := range objects {
		kind := getKindName(object)
		v := reflect.ValueOf(object).Elem()
		v.FieldByName("Kind").SetString(kind)
		items = append(items, &resource.Info{
			Object: object,
		})
	}
	return items
}
