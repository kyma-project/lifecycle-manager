package e2e_test

import (
	apirbacv1 "k8s.io/api/rbac/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("RBAC Privileges", func() {
	Context("Given KCP Cluster with KLM Service Account", func() {
		It("Then KLM Service Account has the correct ClusterRoleBindings", func() {
			klmClusterRoleBindings, err := ListKlmClusterRoleBindings(kcpClient, ctx, "klm-controller-manager")
			Expect(err).ToNot(HaveOccurred())
			Expect(klmClusterRoleBindings.Items).To(HaveLen(1))

			crdRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions/status"},
					Verbs:     []string{"update"},
				},
			}
			Expect(GetClusterRoleBindingPolicyRules(ctx, kcpClient, "klm-controller-manager-crds",
				klmClusterRoleBindings)).To(Equal(crdRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kcp-system namespace")
			kcpSystemKlmRoleBindings, err := ListKlmRoleBindings(kcpClient, ctx, "klm-controller-manager",
				"kcp-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kcpSystemKlmRoleBindings.Items).To(HaveLen(2))

			leaderElectionRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
			}
			Expect(GetRoleBindingRolePolicyRules(ctx, kcpClient, "klm-controller-manager-leader-election",
				"kcp-system",
				kcpSystemKlmRoleBindings)).To(Equal(leaderElectionRoleRules))

			klmManagerRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "patch", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"create", "delete", "get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"services"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions/status"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"cert-manager.io"},
					Resources: []string{"certificates"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"cert-manager.io"},
					Resources: []string{"issuers"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"networking.istio.io"},
					Resources: []string{"gateways"},
					Verbs:     []string{"get", "list"},
				},
				{
					APIGroups: []string{"networking.istio.io"},
					Resources: []string{"virtualservices"},
					Verbs:     []string{"create", "delete", "get", "list", "update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas/status"},
					Verbs:     []string{"get", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"manifests"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"manifests/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"manifests/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"modulereleasemetas"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"moduletemplates"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"moduletemplates/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"watchers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"watchers/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"watchers/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
			}
			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, kcpClient, "klm-controller-manager",
				kcpSystemKlmRoleBindings)).To(Equal(klmManagerRoleRules))

			By("And KLM Service Account has the correct RoleBindings in istio-system namespace")
			istioNamespaceRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"list", "watch", "create", "delete", "update"},
				},
				{
					APIGroups: []string{"cert-manager.io"},
					Resources: []string{"certificates"},
					Verbs:     []string{"patch", "list", "watch", "get", "create", "delete"},
				},
				{
					APIGroups: []string{"cert-manager.io"},
					Resources: []string{"issuers"},
					Verbs:     []string{"list", "watch"},
				},
			}
			istioSystemKlmRoleBindings, err := ListKlmRoleBindings(kcpClient, ctx, "klm-controller-manager",
				"istio-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(istioSystemKlmRoleBindings.Items).To(HaveLen(1))

			Expect(GetRoleBindingRolePolicyRules(ctx, kcpClient, "klm-controller-manager-watcher-certmanager",
				"istio-system",
				istioSystemKlmRoleBindings)).To(Equal(istioNamespaceRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kyma-system namespace")
			remoteNamespaceRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas"},
					Verbs:     []string{"list", "watch", "delete", "get", "create", "patch", "update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas/status"},
					Verbs:     []string{"get", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"moduletemplates"},
					Verbs:     []string{"list", "watch", "delete"},
				},
			}
			kymaSystemKlmRoleBindings, err := ListKlmRoleBindings(kcpClient, ctx, "klm-controller-manager",
				"kyma-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kymaSystemKlmRoleBindings.Items).To(HaveLen(1))

			Expect(GetRoleBindingRolePolicyRules(ctx, kcpClient,
				"klm-controller-manager-skr", "kyma-system",
				kymaSystemKlmRoleBindings)).To(Equal(remoteNamespaceRoleRules))
		})
	})
})
