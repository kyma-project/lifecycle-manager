.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

ifndef GINKGO_FOCUS
$(error GINKGO_FOCUS is not set. Usage: make -f module_deletion_blocking_cluster_scoped_test.mk test "GINKGO_FOCUS=<focus string>")
endif

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Module setup with cluster-scoped CRD"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	cp $(SCRIPTS_DIR)/deploy_moduletemplate.sh .
	cp $(SCRIPTS_DIR)/ocm-config-local-registry.yaml .
	yq eval '.images[0].newTag = "$(MODULE_DEPLOYABLE_VERSION)"' -i config/manager/deployment/kustomization.yaml
	make build-manifests
	crd_selector='select(.kind == "CustomResourceDefinition" and .metadata.name == "samples.operator.kyma-project.io")'
	yq eval "($${crd_selector} | .spec.scope) = \"Cluster\"" -i template-operator.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME) $(MODULE_DEPLOYABLE_VERSION) $(MODULE_DEPLOYABLE_VERSION) true false true false
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_DEPLOYABLE_VERSION)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: $(GINKGO_FOCUS)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "$(GINKGO_FOCUS)"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
