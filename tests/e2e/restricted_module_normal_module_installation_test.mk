.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

DEPLOYER_MODULE_NAME := deployer
# Must match testutils.DeployerDeploymentName so the deployer's Deployment does
# not collide with template-operator's (which uses MODULE_DEPLOYMENT_OLDER_VERSION).
MODULE_DEPLOYMENT_DEPLOYER_VERSION ?= template-operator-deployer-controller-manager
# this global account id is added to the Kyma labels in the test
GLOBAL_ACCOUNT_ID_2 ?= f6e5d4c3-b2a1-9087-6543-210fedcba987

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
# Mark the deployer module as restricted so its installation is gated by the MRM kymaSelector.
# `some-restricted-module` is kept as a sentinel to verify normal module installation is unaffected
# even when a restricted default module is configured that does not exist.
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--restricted-default-modules=$(DEPLOYER_MODULE_NAME),some-restricted-module"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
# template-operator: a normal (non-restricted) module used by the first Describe block.
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh --module-name $(MODULE_NAME) --version $(MODULE_OLDER_VERSION) --deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) --deployable-version $(MODULE_DEPLOYABLE_VERSION)
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
# deployer: a restricted module used by the forced-uninstall Describe block.
# Its kymaSelector initially matches the Kyma's global-account-id label; the test then mutates it.
# Uses its own deployment name so the SKR Deployment does not collide with template-operator's.
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh --module-name $(DEPLOYER_MODULE_NAME) --version $(MODULE_OLDER_VERSION) --deployment-name $(MODULE_DEPLOYMENT_DEPLOYER_VERSION) --deployable-version $(MODULE_DEPLOYABLE_VERSION)
	KEEP_FILE=true $(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(DEPLOYER_MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
	yq -i '.spec.kymaSelector.matchExpressions = [{"key": "kyma-project.io/global-account-id", "operator": "In", "values": ["$(GLOBAL_ACCOUNT_ID_2)"]}]' module-release-meta.yaml
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

	@echo "::group::E2E test: Restricted Modules"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Restricted Modules - "; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
