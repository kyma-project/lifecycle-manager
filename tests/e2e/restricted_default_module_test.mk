.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

RESTRICTED_DEFAULT_MODULES ?= template-operator
GLOBAL_ACCOUNT_ID_1 ?= a1c1d2e3-4a5b-6c7d-8e9f-0a1b2c3d4e5f
# this global account id is added to the Kyma labels in the test
GLOBAL_ACCOUNT_ID_2 ?= f6e5d4c3-b2a1-9087-6543-210fedcba987

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--restricted-default-modules=$(RESTRICTED_DEFAULT_MODULES)"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh --module-name $(MODULE_NAME) --version $(MODULE_OLDER_VERSION) --deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) --deployable-version $(MODULE_DEPLOYABLE_VERSION)
	KEEP_FILE=true $(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
	yq -i '.spec.kymaSelector.matchExpressions = [{"key": "kyma-project.io/global-account-id", "operator": "In", "values": ["$(GLOBAL_ACCOUNT_ID_1)", "$(GLOBAL_ACCOUNT_ID_2)"]}]' module-release-meta.yaml
	kubectl apply -f module-release-meta.yaml
	rm -f module-release-meta.yaml
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Restricted Default Modules"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Restricted Default Modules"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
