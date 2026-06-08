.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	@rm -f purge_finalizer.yaml
	@printf '%s\n' \
		'- op: add' \
		'  path: /spec/template/spec/containers/0/args/-' \
		'  value: --purge-finalizer-timeout=5s' \
		> purge_finalizer.yaml
	@if ! grep -q "path: purge_finalizer.yaml" kustomization.yaml; then kustomize edit add patch --path purge_finalizer.yaml --kind Deployment; fi
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-latest module-setup-in-older-version module-setup-in-newer-version
	@echo "::group::Test-specific ModuleReleaseMeta setup"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_OLDER_VERSION)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Purge Controller"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Purge Controller"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
