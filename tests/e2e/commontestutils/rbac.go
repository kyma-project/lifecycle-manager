package commontestutils

import (
	"context"
	"errors"

	apirbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errFailedToListClusterRoleBindings = errors.New("failed to list ClusterRoleBindings")
	errFailedToListRoleBindings        = errors.New("failed to list RoleBindings")
	errFailedToFetchClusterRole        = errors.New("failed to fetch ClusterRole")
	errFailedToFetchRole               = errors.New("failed to fetch Role")
)

func ListKlmClusterRoleBindings(clnt client.Client, ctx context.Context,
	serviceAccountName string,
) (apirbacv1.ClusterRoleBindingList, error) {
	clusterRoleBindings := apirbacv1.ClusterRoleBindingList{}
	err := clnt.List(ctx, &clusterRoleBindings)
	if err != nil {
		return clusterRoleBindings, errFailedToListClusterRoleBindings
	}
	klmClusterRoleBindings := apirbacv1.ClusterRoleBindingList{}
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
	serviceAccountName, namespace string,
) (apirbacv1.RoleBindingList, error) {
	roleBindings := apirbacv1.RoleBindingList{}
	err := clnt.List(ctx, &roleBindings, client.InNamespace(namespace))
	if err != nil {
		return roleBindings, errFailedToListRoleBindings
	}
	klmRoleBindings := apirbacv1.RoleBindingList{}
	for _, rb := range roleBindings.Items {
		for _, s := range rb.Subjects {
			if s.Kind == "ServiceAccount" && s.Name == serviceAccountName {
				klmRoleBindings.Items = append(klmRoleBindings.Items, rb)
			}
		}
	}

	return klmRoleBindings, nil
}

func GetClusterRolePolicyRules(ctx context.Context, clnt client.Client, roleName string,
	clusterRoleBindings apirbacv1.ClusterRoleBindingList,
) ([]apirbacv1.PolicyRule, error) {
	var policyRules []apirbacv1.PolicyRule
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

func GetRoleBindingRolePolicyRules(ctx context.Context, clnt client.Client, roleName, namespace string,
	roleBindings apirbacv1.RoleBindingList,
) ([]apirbacv1.PolicyRule, error) {
	var policyRules []apirbacv1.PolicyRule
	for _, rb := range roleBindings.Items {
		if rb.RoleRef.Name == roleName {
			role := apirbacv1.Role{}
			err := clnt.Get(ctx, client.ObjectKey{Name: roleName, Namespace: namespace}, &role)
			if err != nil {
				return nil, errFailedToFetchRole
			}
			policyRules = role.Rules
		}
	}
	return policyRules, nil
}

func GetClusterRole(ctx context.Context, clnt client.Client, roleName string) (apirbacv1.ClusterRole, error) {
	clusterRole := apirbacv1.ClusterRole{}
	err := clnt.Get(ctx, client.ObjectKey{Name: roleName}, &clusterRole)
	if err != nil {
		return clusterRole, errFailedToFetchClusterRole
	}
	return clusterRole, nil
}

func getClusterRolePolicyRules(ctx context.Context, clnt client.Client, roleName string) ([]apirbacv1.PolicyRule,
	error,
) {
	clusterRole := apirbacv1.ClusterRole{}
	err := clnt.Get(ctx, client.ObjectKey{Name: roleName}, &clusterRole)
	if err != nil {
		return nil, errFailedToFetchClusterRole
	}
	return clusterRole.Rules, nil
}
