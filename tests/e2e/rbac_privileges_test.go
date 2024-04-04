package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/rbac/v1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("RBAC Privileges", func() {
	Context("Given KCP Cluster with KLM Service Account", func() {
		It("Then KLM Service Account has the correct ClusterRoleBindings", func() {
			klmClusterRoleBindings, err := ListKlmClusterRoleBindings(controlPlaneClient, ctx, "klm-controller-manager")
			Expect(err).ToNot(HaveOccurred())
			Expect(klmClusterRoleBindings.Items).To(HaveLen(2))

			gatewayRoleRules := []v1.PolicyRule{
				{
					APIGroups: []string{"networking.istio.io"},
					Resources: []string{"gateways"},
					Verbs:     []string{"get", "list"},
				},
			}
			Expect(GetClusterRoleBindingPolicyRules(ctx, controlPlaneClient, "klm-manager-role-gateway",
				klmClusterRoleBindings)).To(Equal(gatewayRoleRules))

			crdRoleRules := []v1.PolicyRule{
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
			Expect(GetClusterRoleBindingPolicyRules(ctx, controlPlaneClient, "klm-manager-role-crd",
				klmClusterRoleBindings)).To(Equal(crdRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kcp-system namespaces")
			kcpSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"kcp-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kcpSystemKlmRoleBindings.Items).To(HaveLen(3))

			leaderElectionRoleRules := []v1.PolicyRule{
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
			Expect(GetRoleBindingRolePolicyRules(ctx, controlPlaneClient, "klm-leader-election-role", "kcp-system",
				kcpSystemKlmRoleBindings)).To(Equal(leaderElectionRoleRules))

			klmManagerRoleRules := []v1.PolicyRule{
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
			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-role",
				kcpSystemKlmRoleBindings)).To(Equal(klmManagerRoleRules))

			manifestRoleRules := []v1.PolicyRule{
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
			}
			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-manifest",
				kcpSystemKlmRoleBindings)).To(Equal(manifestRoleRules))

			By("And KLM Service Account has the correct RoleBindings in istio-system namespaces")
			istioSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"istio-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(istioSystemKlmRoleBindings.Items).To(HaveLen(2))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-manifest",
				istioSystemKlmRoleBindings)).To(Equal(manifestRoleRules))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-role",
				istioSystemKlmRoleBindings)).To(Equal(klmManagerRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kyma-system namespaces")
			kymaSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"kyma-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kymaSystemKlmRoleBindings.Items).To(HaveLen(3))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-manifest",
				kymaSystemKlmRoleBindings)).To(Equal(manifestRoleRules))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-role",
				kymaSystemKlmRoleBindings)).To(Equal(klmManagerRoleRules))

			metricsReaderRoleRules := []v1.PolicyRule{
				{
					NonResourceURLs: []string{"/metrics"},
					Verbs:           []string{"get"},
				},
			}
			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-metrics-reader",
				kymaSystemKlmRoleBindings)).To(Equal(metricsReaderRoleRules))
		})
	})
})
