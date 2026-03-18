.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	### Add Changes Here
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	if [ -f $(MANDATORY_TEMPLATE_V2) ]; then echo "ERROR: $(MANDATORY_TEMPLATE_V2) already exists. Run 'make clean-test-artifacts' first."; exit 1; fi
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	### Add Changes Here
	cp template.yaml $(MANDATORY_TEMPLATE_V2)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Mandatory Module Installation and Deletion"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	### Change Test Name
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "CHANGE ME!"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
