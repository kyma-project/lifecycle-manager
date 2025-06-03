package testutils

import (
	"context"
	"fmt"

	apinetworkv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NetworkPolicyExists(ctx context.Context, clnt client.Client, name, namespace string) error {
	networkPolicy, err := GetNetworkPolicy(ctx, clnt, name, namespace)
	return CRExists(networkPolicy, err)
}

func GetNetworkPolicy(ctx context.Context, clnt client.Client, name, namespace string) (*apinetworkv1.NetworkPolicy,
	error,
) {
	resource := &apinetworkv1.NetworkPolicy{}

	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, resource)
	if err != nil {
		return nil, fmt.Errorf("get networkpolicy: %w", err)
	}
	return resource, nil
}
