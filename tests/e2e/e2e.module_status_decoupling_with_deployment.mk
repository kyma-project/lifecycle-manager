.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Module setup (status decoupling with deployment)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	cp $(SCRIPTS_DIR)/deploy_moduletemplate.sh .
	cp $(SCRIPTS_DIR)/ocm-config-local-registry.yaml .

	# Warning deployment variant
	yq eval '.images[0].newTag = "$(VERSION_WARNING_DEPLOYMENT)"' -i config/manager/deployment/kustomization.yaml
	make build-manifests
	yq eval '(. | select(.kind == "Deployment") | .spec.template.spec.containers[0].args) = ["--leader-elect", "--final-state=Warning", "--final-deletion-state=Warning"]' -i template-operator.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME) $(VERSION_WARNING_DEPLOYMENT) $(MODULE_DEPLOYABLE_VERSION) true false true false
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(VERSION_WARNING_DEPLOYMENT)

	# Misconfigured deployment variant
	yq eval '.images[0].newTag = "$(VERSION_MISCONFIGURED_DEPLOYMENT)"' -i config/manager/deployment/kustomization.yaml
	make build-manifests
	yq eval '(. | select(.kind == "Deployment") | .spec.template.spec.containers[0].image) = "non-working/path:0.0.1"' -i template-operator.yaml
	yq eval '(. | select(.kind == "Deployment") | .spec.progressDeadlineSeconds) = 60' -i template-operator.yaml
	yq eval '(. | select(.kind == "Deployment") | .spec.template.spec.containers[0].livenessProbe) = {"httpGet": {"path": "/healthz", "port": 8081}, "initialDelaySeconds": 5, "periodSeconds": 5, "failureThreshold": 1}' -i template-operator.yaml
	yq eval '(. | select(.kind == "Deployment") | .spec.template.spec.containers[0].readinessProbe) = {"httpGet": {"path": "/readyz", "port": 8081}, "initialDelaySeconds": 5, "periodSeconds": 5, "failureThreshold": 1}' -i template-operator.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME_MISCONFIGURED) $(VERSION_MISCONFIGURED_DEPLOYMENT) $(MODULE_DEPLOYABLE_VERSION) true false true false
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME_MISCONFIGURED) regular:$(VERSION_MISCONFIGURED_DEPLOYMENT)

	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Module Status Decoupling With Deployment"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Module Status Decoupling With Deployment"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
