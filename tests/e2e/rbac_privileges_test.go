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
			klmClusterRoleBindings, err := ListKlmClusterRoleBindings(controlPlaneClient, ctx, "klm-controller-manager")
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
			Expect(GetClusterRoleBindingPolicyRules(ctx, controlPlaneClient, "klm-manager-role-crd",
				klmClusterRoleBindings)).To(Equal(crdRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kcp-system namespace")
			kcpSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"kcp-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kcpSystemKlmRoleBindings.Items).To(HaveLen(3))

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
			Expect(GetRoleBindingRolePolicyRules(ctx, controlPlaneClient, "klm-leader-election-role", "kcp-system",
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

			manifestRoleRules := []apirbacv1.PolicyRule{
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
			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-role-manifest",
				kcpSystemKlmRoleBindings)).To(Equal(manifestRoleRules))

			By("And KLM Service Account has the correct RoleBindings in istio-system namespace")
			istioNamespaceRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"cert-manager.io"},
					Resources: []string{"certificates"},
					Verbs:     []string{"patch", "list", "watch"},
				},
			}
			istioSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"istio-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(istioSystemKlmRoleBindings.Items).To(HaveLen(1))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient, "klm-manager-role-istio-namespace",
				istioSystemKlmRoleBindings)).To(Equal(istioNamespaceRoleRules))

			By("And KLM Service Account has the correct RoleBindings in kyma-system namespace")
			remoteNamespaceRoleRules := []apirbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"kymas"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"operator.kyma-project.io"},
					Resources: []string{"moduletemplates"},
					Verbs:     []string{"list", "watch"},
				},
			}
			kymaSystemKlmRoleBindings, err := ListKlmRoleBindings(controlPlaneClient, ctx, "klm-controller-manager",
				"kyma-system")
			Expect(err).ToNot(HaveOccurred())
			Expect(kymaSystemKlmRoleBindings.Items).To(HaveLen(2))

			Expect(GetRoleBindingwithClusterRolePolicyRules(ctx, controlPlaneClient,
				"klm-manager-role-remote-namespace",
				kymaSystemKlmRoleBindings)).To(Equal(remoteNamespaceRoleRules))

			metricsReaderRoleRules := []apirbacv1.PolicyRule{
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
