.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

ifndef GINKGO_FOCUS
$(error GINKGO_FOCUS is not set. Usage: make -f maintenance_windows_test.mk test "GINKGO_FOCUS=<focus string>")
endif

.PHONY: klm-patch
klm-patch:
	@echo "::group::Maintenance-window policy"
	@$(SCRIPTS_DIR)/generate_maintenance_window_policy.sh \
		$(LIFECYCLE_MANAGER_DIR)/config/maintenance_windows/policy.json
	@echo "::endgroup::"

.PHONY: module-setup-in-newer-version-requires-downtime
module-setup-in-newer-version-requires-downtime:
	@echo "::group::Module setup with $(MODULE_NEWER_VERSION) (requires-downtime)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(MODULE_NEWER_VERSION) \
		--deployment-name $(MODULE_DEPLOYMENT_NEWER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION) \
		--requires-downtime
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-in-older-version module-setup-in-newer-version-requires-downtime
	@echo "::group::Test-specific ModuleReleaseMeta setup"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_NEWER_VERSION) regular:$(MODULE_OLDER_VERSION)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: $(GINKGO_FOCUS)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "$(GINKGO_FOCUS)"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
