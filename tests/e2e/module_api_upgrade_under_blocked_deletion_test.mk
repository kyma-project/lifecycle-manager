.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch"
	@echo "No test-specific KLM patches"
	@echo "::endgroup::"

.PHONY: module-setup-in-older-version-e2e
module-setup-in-older-version-e2e:
	@echo "::group::Module setup with $(MODULE_OLDER_VERSION_E2E)"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh \
		--module-name $(MODULE_NAME) \
		--version $(MODULE_OLDER_VERSION_E2E) \
		--deployment-name $(MODULE_DEPLOYMENT_OLDER_VERSION) \
		--deployable-version $(MODULE_DEPLOYABLE_VERSION)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup-in-newer-version-e2e-with-crd-upgrade
module-setup-in-newer-version-e2e-with-crd-upgrade:
	@echo "::group::Module setup with $(MODULE_NEWER_VERSION_E2E) and CRD v1beta1 upgrade"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	yq eval '.apiVersion = "operator.kyma-project.io/v1alpha1"' -i config/samples/default-sample-cr.yaml
	cp $(SCRIPTS_DIR)/deploy_moduletemplate.sh .
	yq eval '.images[0].newTag = "$(MODULE_NEWER_VERSION_E2E)"' -i config/manager/deployment/kustomization.yaml
	make build-manifests
	yq eval '(. | select(.kind == "Deployment") | .metadata.name) = "$(MODULE_DEPLOYMENT_NEWER_VERSION)"' -i template-operator.yaml
	crd_selector='select(.kind == "CustomResourceDefinition" and .metadata.name == "samples.operator.kyma-project.io")'
	yq eval "$${crd_selector} |= (.spec.versions += [{\"name\": \"v1beta1\", \"served\": true, \"storage\": true, \"schema\": .spec.versions[0].schema}])" -i template-operator.yaml
	yq eval "($${crd_selector} | .spec.versions[] | select(.name == \"v1alpha1\")).storage = false" -i template-operator.yaml
	yq eval '.apiVersion = "operator.kyma-project.io/v1beta1"' -i config/samples/default-sample-cr.yaml
	./deploy_moduletemplate.sh $(MODULE_NAME) $(MODULE_NEWER_VERSION_E2E) $(MODULE_DEPLOYABLE_VERSION) true false true false
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup: module-setup-in-older-version-e2e module-setup-in-newer-version-e2e-with-crd-upgrade
	@echo "::group::Test-specific module metadata setup"
	@export PATH=$(LOCALBIN):$$PATH
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) fast:$(MODULE_NEWER_VERSION_E2E) regular:$(MODULE_OLDER_VERSION_E2E)
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: Module API Upgrade Under Blocked Deletion"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Module API Upgrade Under Blocked Deletion"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
