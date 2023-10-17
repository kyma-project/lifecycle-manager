package testutils

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TestModuleCRName = "sample-yaml"
)

var (
	errSampleCRDeletionTimestampSet    = errors.New("sample CR has set DeletionTimeStamp")
	errSampleCRDeletionTimestampNotSet = errors.New("sample CR has not set DeletionTimeStamp")
)

func ModuleCRExists(ctx context.Context, clnt client.Client, moduleCR *unstructured.Unstructured) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: moduleCR.GetNamespace(),
		Name:      moduleCR.GetName(),
	}, moduleCR)
	return CRExists(moduleCR, err)
}

// NewTestModuleCR init one module cr used by template-operator.
func NewTestModuleCR(namespace string) *unstructured.Unstructured {
	return builder.NewModuleCRBuilder().
		WithName(TestModuleCRName).
		WithNamespace(namespace).Build()
}

func SampleCRNoDeletionTimeStampSet(ctx context.Context, name, namespace string, clnt client.Client) error {
	_, exists, err := GetDeletionTimeStamp(ctx, "operator.kyma-project.io", "v1alpha1",
		"Sample", name, namespace, clnt)
	if err != nil {
		return err
	}

	if exists {
		return errSampleCRDeletionTimestampSet
	}
	return nil
}

func SampleCRDeletionTimeStampSet(ctx context.Context, name, namespace string, clnt client.Client) error {
	_, exists, err := GetDeletionTimeStamp(ctx, "operator.kyma-project.io", "v1alpha1",
		"Sample", name, namespace, clnt)
	if err != nil {
		return err
	}

	if !exists {
		return errSampleCRDeletionTimestampNotSet
	}
	return nil
}
