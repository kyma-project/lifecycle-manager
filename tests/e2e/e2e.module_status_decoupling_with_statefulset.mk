.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

VERSION_WARNING_STATEFULSET       := $(MODULE_VERSION_OLDER_BASE)-warning-statefulset
VERSION_MISCONFIGURED_STATEFULSET := $(MODULE_VERSION_OLDER_BASE)-misconfigured-statefulset
MODULE_NAME_MISCONFIGURED         := template-operator-misconfigured

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Module setup (status decoupling with statefulset)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	cp $(SCRIPTS_DIR)/deploy_moduletemplate.sh .
	cp $(SCRIPTS_DIR)/ocm-config-local-registry.yaml .

	# Warning statefulset variant
	yq eval '.images[0].newTag = "$(VERSION_WARNING_STATEFULSET)"' -i config/manager/statefulset/kustomization.yaml
	make build-statefulset-manifests
	yq eval '(. | select(.kind == "StatefulSet") | .spec.template.spec.containers[0].args) = ["--leader-elect", "--final-state=Warning", "--final-deletion-state=Warning"]' -i template-operator.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME) $(VERSION_WARNING_STATEFULSET) $(MODULE_DEPLOYABLE_VERSION) true false true false
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(VERSION_WARNING_STATEFULSET)

	# Misconfigured statefulset variant
	yq eval '.images[0].newTag = "$(VERSION_MISCONFIGURED_STATEFULSET)"' -i config/manager/statefulset/kustomization.yaml
	make build-statefulset-manifests
	yq eval '(. | select(.kind == "StatefulSet") | .spec.template.spec.containers[0].image) = "non-working/path:0.0.2"' -i template-operator.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME_MISCONFIGURED) $(VERSION_MISCONFIGURED_STATEFULSET) $(MODULE_DEPLOYABLE_VERSION) true false true false
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME_MISCONFIGURED) regular:$(VERSION_MISCONFIGURED_STATEFULSET)

	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Module Status Decoupling With StatefulSet"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Module Status Decoupling With StatefulSet"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
