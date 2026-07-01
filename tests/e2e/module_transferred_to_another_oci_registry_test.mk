.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch - remote OCI registry host"
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test/patches > /dev/null
	printf '%s\n' \
		'- op: add' \
		'  path: /spec/template/spec/containers/0/args/-' \
		'  value: --oci-registry-host=https://europe-west3-docker.pkg.dev' \
		'- op: add' \
		'  path: /spec/template/spec/containers/0/args/-' \
		'  value: --modules-repository-subpath=sap-kyma-jellyfish-dev/restricted-market' \
		> oci_registry_host.yaml
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Module setup"
	@export PATH=$(LOCALBIN):$$PATH
	kubectl apply -f $(E2E_TESTS_DIR)/moduletemplate/moduletemplate_template_operator_transferred.yaml
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_DEPLOYABLE_VERSION)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Module Transferred to Another OCI Registry"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Module Transferred to Another OCI Registry"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
