package testutils

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

func DeleteCR(ctx context.Context, clnt client.Client, obj client.Object) error {
	err := clnt.Delete(ctx, obj)
	if err != nil && util.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := clnt.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj); err != nil {
		if util.IsNotFound(err) {
			return nil
		}
		return err
	}
	return fmt.Errorf("%s/%s: %w", obj.GetNamespace(), obj.GetName(), ErrNotDeleted)
}

func DeleteCRWithGVK(ctx context.Context, clnt client.Client, name, namespace, group, version, kind string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return DeleteCR(ctx, clnt, obj)
}

func GetCR(ctx context.Context, clnt client.Client,
	objectKey client.ObjectKey,
	gvk schema.GroupVersionKind,
) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err := clnt.Get(ctx, objectKey, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func IsResourceVersionSame(ctx context.Context, clnt client.Client,
	objectKey client.ObjectKey,
	gvk schema.GroupVersionKind, expectedVersion string,
) (bool, error) {
	obj, err := GetCR(ctx, clnt, objectKey, gvk)
	if err != nil {
		return false, err
	}
	if obj.GetResourceVersion() == expectedVersion {
		return true, nil
	}
	return false, nil
}

func CreateCR(ctx context.Context, clnt client.Client, obj client.Object) error {
	err := clnt.Create(ctx, obj)
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func UpdateCR(ctx context.Context, clnt client.Client, obj client.Object) error {
	return clnt.Update(ctx, obj)
}

func CRExists(obj apimetav1.Object, clientError error) error {
	if util.IsNotFound(clientError) {
		return ErrNotFound
	}
	if clientError != nil {
		return clientError
	}
	if obj != nil && obj.GetDeletionTimestamp() != nil {
		return ErrDeletionTimestampFound
	}
	if obj == nil {
		return ErrNotFound
	}
	return nil
}

func CRIsInState(ctx context.Context, group, version, kind, name, namespace string, statusPath []string,
	clnt client.Client, expectedState string,
) error {
	resourceCR, err := GetCR(ctx, clnt, client.ObjectKey{Name: name, Namespace: namespace}, schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	if err != nil {
		return err
	}

	stateFromCR, stateExists, err := unstructured.NestedString(resourceCR.Object, statusPath...)
	if err != nil || !stateExists {
		return ErrFetchingStatus
	}

	if stateFromCR != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			ErrSampleCrNotInExpectedState, expectedState, stateFromCR)
	}
	return nil
}
