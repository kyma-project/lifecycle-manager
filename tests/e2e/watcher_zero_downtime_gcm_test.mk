.DEFAULT_GOAL := test-run
.PHONY: test-run $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch: kustomize-install
	@echo "::group::KLM patch"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test_gcm > /dev/null
	# op: replace at index 11 targets --kyma-requeue-success-interval added by config/watcher_local_test_gcm/kustomization.yaml.
	# Verify the arg index there if this patch breaks silently after changing the base args list.
	kustomize edit add patch --kind Deployment --patch \
		'[{"op":"replace","path":"/spec/template/spec/containers/0/args/11","value":"--kyma-requeue-success-interval=1s"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--istio-gateway-server-cert-switch-grace-period=30s"},{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--istio-gateway-secret-requeue-success-interval=1s"}]'
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Watcher Zero Downtime"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Watcher Zero Downtime"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}
