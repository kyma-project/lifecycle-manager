package testutils

import (
	"context"
	"errors"

	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errFailedToListClusterRoleBindings = errors.New("failed to list ClusterRoleBindings")
	errFailedToListRoleBindings        = errors.New("failed to list RoleBindings")
	errFailedToFetchClusterRole        = errors.New("failed to fetch ClusterRole")
	errFailedToFetchRole               = errors.New("failed to fetch Role")
	errContainedUnexpectedRule         = errors.New("cluster role contained unexpected rule")
)

func ListKlmClusterRoleBindings(clnt client.Client, ctx context.Context,
	serviceAccountName string) (v1.ClusterRoleBindingList, error) {
	clusterRoleBindings := v1.ClusterRoleBindingList{}
	err := clnt.List(ctx, &clusterRoleBindings)
	if err != nil {
		return clusterRoleBindings, errFailedToListClusterRoleBindings
	}
	klmClusterRoleBindings := v1.ClusterRoleBindingList{}
	for _, rb := range clusterRoleBindings.Items {
		for _, s := range rb.Subjects {
			if s.Kind == "ServiceAccount" && s.Name == serviceAccountName {
				klmClusterRoleBindings.Items = append(klmClusterRoleBindings.Items, rb)
			}
		}
	}

	return klmClusterRoleBindings, nil
}

func ListKlmRoleBindings(clnt client.Client, ctx context.Context,
	serviceAccountName, namespace string) (v1.RoleBindingList, error) {
	roleBindings := v1.RoleBindingList{}
	err := clnt.List(ctx, &roleBindings, client.InNamespace(namespace))
	if err != nil {
		return roleBindings, errFailedToListRoleBindings
	}
	klmRoleBindings := v1.RoleBindingList{}
	for _, rb := range roleBindings.Items {
		for _, s := range rb.Subjects {
			if s.Kind == "ServiceAccount" && s.Name == serviceAccountName {
				klmRoleBindings.Items = append(klmRoleBindings.Items, rb)
			}
		}
	}

	return klmRoleBindings, nil
}

func GetClusterRoleBindingPolicyRules(ctx context.Context, clnt client.Client, roleName string,
	clusterRoleBindings v1.ClusterRoleBindingList) ([]v1.PolicyRule, error) {
	var policyRules []v1.PolicyRule
	for _, crb := range clusterRoleBindings.Items {
		if crb.RoleRef.Name == roleName {
			var err error
			policyRules, err = getClusterRolePolicyRules(ctx, clnt, roleName)
			if err != nil {
				return nil, errFailedToFetchClusterRole
			}
		}
	}
	return policyRules, nil
}

func GetRoleBindingwithClusterRolePolicyRules(ctx context.Context, clnt client.Client, roleName string,
	roleBindings v1.RoleBindingList) ([]v1.PolicyRule, error) {
	var policyRules []v1.PolicyRule
	for _, rb := range roleBindings.Items {
		if rb.RoleRef.Name == roleName {
			var err error
			policyRules, err = getClusterRolePolicyRules(ctx, clnt, roleName)
			if err != nil {
				return nil, errFailedToFetchClusterRole
			}
		}
	}
	return policyRules, nil
}

func GetRoleBindingRolePolicyRules(ctx context.Context, clnt client.Client, roleName, namespace string,
	roleBindings v1.RoleBindingList) ([]v1.PolicyRule, error) {
	var policyRules []v1.PolicyRule
	for _, rb := range roleBindings.Items {
		if rb.RoleRef.Name == roleName {
			role := v1.Role{}
			err := clnt.Get(ctx, client.ObjectKey{Name: roleName, Namespace: namespace}, &role)
			if err != nil {
				return nil, errFailedToFetchRole
			}
			policyRules = role.Rules
		}
	}
	return policyRules, nil
}

func getClusterRolePolicyRules(ctx context.Context, clnt client.Client, roleName string) ([]v1.PolicyRule, error) {
	clusterRole := v1.ClusterRole{}
	err := clnt.Get(ctx, client.ObjectKey{Name: roleName}, &clusterRole)
	if err != nil {
		return nil, errFailedToFetchClusterRole
	}
	return clusterRole.Rules, nil
}
