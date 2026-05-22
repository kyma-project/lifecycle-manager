.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

VERSION_NO_DEFAULT_CR := 1.0.0-no-default-cr

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Module setup (no default CR)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(VERSION_NO_DEFAULT_CR) \
		--deployment-name $(MODULE_DEPLOYMENT_CURRENT_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION) \
		--no-default-cr
	@popd > /dev/null
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(VERSION_NO_DEFAULT_CR)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Module Without Default CR"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Module Without Default CR"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
