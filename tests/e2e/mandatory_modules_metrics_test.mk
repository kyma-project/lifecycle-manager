.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh --module-name template-operator --version 1.1.0-smoke-test --deployment-name template-operator-v1-controller-manager --deployable-version 1.0.4 --mandatory
	$(SCRIPTS_DIR)/deploy_mandatory_modulereleasemeta.sh template-operator 1.1.0-smoke-test
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Mandatory Module Metrics"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Mandatory Module Metrics"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
