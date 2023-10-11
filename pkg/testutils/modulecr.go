package testutils

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TestModuleCRName = "sample-yaml"

func ModuleCRExists(ctx context.Context, clnt client.Client, name, namespace string) error {
	moduleCR := unstructured.Unstructured{}
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &moduleCR)
	return CRExists(&moduleCR, err)
}

// NewTestModuleCR init one module cr used by template-operator
func NewTestModuleCR(namespace string) *unstructured.Unstructured {
	return builder.NewModuleCRBuilder().
		WithName(TestModuleCRName).
		WithNamespace(namespace).Build()
}
