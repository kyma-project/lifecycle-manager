.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
# Support normal module installation, although a restricted default module is defined, that does not exist
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--restricted-default-modules=some-restricted-module"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh --module-name $(MODULE_NAME) --version $(MODULE_OLDER_VERSION) --deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) --deployable-version $(MODULE_DEPLOYABLE_VERSION)
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Restricted Modules - Normal Module Installation"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Restricted Modules - Normal Module Installation"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
