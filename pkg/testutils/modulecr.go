package testutils

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ModuleCRExists(ctx context.Context, clnt client.Client, name, namespace string) error {
	moduleCR := unstructured.Unstructured{}
	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &moduleCR)
	return CRExists(&moduleCR, err)
}
