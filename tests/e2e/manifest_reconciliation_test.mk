.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: module-setup
module-setup: module-setup-latest module-setup-in-older-version module-setup-in-newer-version
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_DEPLOYABLE_VERSION)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: klm-patch
klm-patch: kustomize-install ## Patch KLM deployment with high concurrency to match private cloud setup.
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--max-concurrent-manifest-reconciles=75"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--max-concurrent-kyma-reconciles=25"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Manifest Skip Reconciliation Label"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Manifest Skip Reconciliation Label"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
