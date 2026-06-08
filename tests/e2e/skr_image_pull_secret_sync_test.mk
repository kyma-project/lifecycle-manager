.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	@rm -f skr_image_pull_secret_patch.yaml
	@printf '%s\n' \
		'- op: add' \
		'  path: /spec/template/spec/containers/0/args/-' \
		'  value: --skr-image-pull-secret=some-special-image-pull-secret' \
		> skr_image_pull_secret_patch.yaml
	@if ! grep -q "path: skr_image_pull_secret_patch.yaml" kustomization.yaml; then kustomize edit add patch --path skr_image_pull_secret_patch.yaml --kind Deployment; fi
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-latest module-setup-in-older-version module-setup-in-newer-version
	@echo "::group::Test-specific ModuleReleaseMeta setup"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_NEWER_VERSION) regular:$(MODULE_OLDER_VERSION)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: SKR Image Pull Secret Sync"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "SKR Image Pull Secret Sync"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
