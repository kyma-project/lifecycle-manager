# standing_env.mk – bring up a long-running local E2E environment for manual inspection.
#
# Unlike the *_test.mk files, this makefile intentionally does NOT run any Ginkgo
# test and does NOT tear down the clusters. It leaves you with:
#   - a KCP k3d cluster running KLM
#   - one SKR k3d cluster
#   - an access Secret + Kyma CR (kyma-sample) wired to that SKR
#   - a single mandatory template-operator module (ModuleTemplate + mandatory ModuleReleaseMeta)
#
# Bring it up:
#   make -f tests/e2e/standing_env.mk up
#
# Inspect it afterwards (see the `info` target for handy commands):
#   make -f tests/e2e/standing_env.mk info
#
# When you're done, tear it down explicitly (from e2e.common.mk):
#   make -f tests/e2e/standing_env.mk teardown

.DEFAULT_GOAL := up
.PHONY: up $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Mandatory module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(MODULE_MANDATORY_OLDER_VERSION) \
		--deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION) \
		--mandatory
	$(SCRIPTS_DIR)/deploy_mandatory_modulereleasemeta.sh $(MODULE_NAME) $(MODULE_MANDATORY_OLDER_VERSION)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: deploy-kyma
deploy-kyma:
	@echo "::group::Deploy Kyma CR + SKR access secret (kyma-sample)"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_kyma.sh host.k3d.internal
	@echo "::endgroup::"

.PHONY: info
info:
	@echo "==== Standing E2E environment ===="
	@echo "KCP kubeconfig: $$(k3d kubeconfig write kcp)"
	@echo "SKR kubeconfig: $$(k3d kubeconfig write skr)"
	@echo
	@echo "Inspect KCP (KLM, Kyma CR, Manifests, module metadata):"
	@echo "  export KUBECONFIG=\$$(k3d kubeconfig write kcp)"
	@echo "  kubectl -n kcp-system get kyma kyma-sample -o yaml"
	@echo "  kubectl -n kcp-system get manifest -o wide"
	@echo "  kubectl -n kcp-system get moduletemplate,modulereleasemeta -o wide"
	@echo "  kubectl -n kcp-system logs deploy/klm-controller-manager -f"
	@echo
	@echo "Inspect SKR (deployed module resources):"
	@echo "  export KUBECONFIG=\$$(k3d kubeconfig write skr)"
	@echo "  kubectl -n kyma-system get all"
	@echo
	@echo "Tear down when done:"
	@echo "  make -f tests/e2e/standing_env.mk teardown"

# Full bring-up, no test-run, no teardown.
.PHONY: up
up: create-clusters klm-patch deploy-klm module-setup deploy-kyma info
