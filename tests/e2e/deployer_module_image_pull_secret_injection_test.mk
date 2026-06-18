.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

MODULE_NAME := deployer
GLOBAL_ACCOUNT_ID ?= a1c1d2e3-4a5b-6c7d-8e9f-0a1b2c3d4e5f

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--restricted-default-modules=$(MODULE_NAME)"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(MODULE_OLDER_VERSION) \
		--deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION) \
		--additional-resources $(E2E_TESTS_DIR)/testdata/skr-deployer-image-pull-secret.yaml \
		--additional-resources $(E2E_TESTS_DIR)/testdata/skr-deployer-non-inject-pull-secret.yaml
	KEEP_FILE=true $(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_OLDER_VERSION) regular:$(MODULE_OLDER_VERSION)
	yq -i '.spec.kymaSelector.matchExpressions = [{"key": "kyma-project.io/global-account-id", "operator": "In", "values": ["$(GLOBAL_ACCOUNT_ID)"]}]' module-release-meta.yaml
	kubectl apply -f module-release-meta.yaml
	rm -f module-release-meta.yaml
# KCP secret: actual data that KLM should inject into the deployer module's secret on SKR
	kubectl apply -f $(E2E_TESTS_DIR)/testdata/kcp-deployer-image-pull-secret.yaml
# KCP secret with matching name but no annotation on the SKR counterpart — should not be injected
	kubectl apply -f $(E2E_TESTS_DIR)/testdata/kcp-deployer-non-inject-pull-secret.yaml
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Deployer Module Image Pull Secret Injection"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Deployer Module Image Pull Secret Injection"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
