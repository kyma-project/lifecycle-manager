.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

MODULE_NAME := not-deployer
DEPLOYER_MODULE_NAME := deployer
GLOBAL_ACCOUNT_ID ?= f6e5d4c3-b2a1-9087-6543-210fedcba987

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--restricted-default-modules=deployer,$(MODULE_NAME)"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
# not-deployer module fulfills the criteria for image pull secret injection;
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(MODULE_OLDER_VERSION) \
		--deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION) \
		--additional-resources $(E2E_TESTS_DIR)/testdata/skr-deployer-image-pull-secret.yaml
	KEEP_FILE=true $(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
	yq -i '.spec.kymaSelector.matchExpressions = [{"key": "kyma-project.io/global-account-id", "operator": "In", "values": ["$(GLOBAL_ACCOUNT_ID)"]}]' module-release-meta.yaml
	kubectl apply -f module-release-meta.yaml
	rm -f module-release-meta.yaml
# deployer module is required to exist because it is configured as a restricted default module;
# its kymaSelector uses a non-matching global-account-id so it is not installed for the test Kyma CR
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(DEPLOYER_MODULE_NAME) \
		--version $(MODULE_OLDER_VERSION) \
		--deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION)
	KEEP_FILE=true $(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(DEPLOYER_MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
	yq -i '.spec.kymaSelector.matchExpressions = [{"key": "kyma-project.io/global-account-id", "operator": "In", "values": ["00000000-0000-0000-0000-000000000000"]}]' module-release-meta.yaml
	kubectl apply -f module-release-meta.yaml
	rm -f module-release-meta.yaml
# KCP secret: data that should NOT be injected because the module is not named "deployer"
	kubectl apply -f $(E2E_TESTS_DIR)/testdata/kcp-not-deployer-image-pull-secret.yaml
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Block Non Deployer Module Image Pull Secret Injection"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Block Non Deployer Module Image Pull Secret Injection"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
