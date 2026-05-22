.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	# op: replace at index 11 targets --kyma-requeue-success-interval added by config/watcher_local_test/kustomization.yaml.
	# Verify the arg index there if this patch breaks silently after changing the base args list.
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"replace","path":"/spec/template/spec/containers/0/args/11","value":"--kyma-requeue-success-interval=1h"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kyma-requeue-warning-interval=1h"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kyma-requeue-error-interval=1h"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kyma-requeue-busy-interval=1s"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--module-upgrade-rollout-max-delay=10s"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-latest
	@echo "::group::Module setup for modulereleasemeta-watch-trigger"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_DEPLOYABLE_VERSION)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: ModuleReleaseMeta Watch Trigger"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "ModuleReleaseMeta Watch Trigger"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
