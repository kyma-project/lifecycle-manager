.DEFAULT_GOAL := test

# Mark all the targets provided in the command line and "test" as PHONY ones.
.PHONY: test $(MAKECMDGOALS)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Go command with FIPS140 Module enabled
GO := GOFIPS140=v1.0.0 go

### Important directories.
LOCALBIN ?= $(E2E_TESTS_DIR)/bin
LIFECYCLE_MANAGER_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../)
E2E_TESTS_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/tests/e2e/)
SCRIPTS_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/scripts/tests/)
INSTALL_SCRIPTS_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/scripts/install/)
TEMPLATE_OPERATOR_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/../template-operator/)


### Tools versions.

# Note there's no "v" prefix in the versions.
GINKGO_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_ginkgo_version.sh)
K8S_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_k8s_version.sh)
CERT-MANAGER-VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_cert_manager_version.sh)
ISTIOCTL_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_istioctl_version.sh)
KUSTOMIZE_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_kustomize_version.sh)
MODULECTL_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_modulectl_version.sh)
OCM_VERSION ?= $(shell ${INSTALL_SCRIPTS_DIR}/required_ocm_version.sh)


### Test-specific configurations.

MODULE_NAME ?= template-operator
OLDER_VERSION_FOR_MANDATORY_MODULE ?= 1.1.0-smoke-test
#NEWER_VERSION_FOR_MANDATORY_MODULE ?= 2.4.1-smoke-test
MODULE_DEPLOYMENT_NAME_IN_OLDER_VERSION ?= template-operator-v1-controller-manager
#MODULE_DEPLOYMENT_NAME_IN_NEWER_VERSION ?= template-operator-v2-controller-manager
DEPLOYABLE_MODULE_VERSION ?= 1.0.4

# Ginkgo binary metadata.
GINKGO_CMD ?= $(LOCALBIN)/ginkgo


# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
.ONESHELL:
SHELL := bash
.SHELLFLAGS = -o pipefail -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

# Location to install local Go binaries to.
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)


## install necessary binaries in LOCALBIN.

ginkgo-install: $(LOCALBIN)
	@pushd $(LOCALBIN) > /dev/null
	$(INSTALL_SCRIPTS_DIR)/ginkgo_install.sh $(GINKGO_VERSION)
	@popd > /dev/null

kustomize-install: $(LOCALBIN)
	@pushd $(LOCALBIN) > /dev/null
	@${INSTALL_SCRIPTS_DIR}/kustomize_install.sh $(KUSTOMIZE_VERSION)
	@popd > /dev/null

modulectl-install: $(LOCALBIN)
	@pushd $(LOCALBIN) > /dev/null
	@${INSTALL_SCRIPTS_DIR}/modulectl_install.sh $(MODULECTL_VERSION)
	@popd > /dev/null

istioctl-install: $(LOCALBIN)
	@pushd $(LOCALBIN) > /dev/null
	@${INSTALL_SCRIPTS_DIR}/istioctl_install.sh $(ISTIOCTL_VERSION)
	@popd > /dev/null

ocm-install: $(LOCALBIN)
	@pushd $(LOCALBIN) > /dev/null
	$(INSTALL_SCRIPTS_DIR)/ocm_install.sh $(OCM_VERSION)
	@popd > /dev/null


e2e-coverage: ginkgo-install ## Generate the effective Acceptance Criteria for all the test suites.
	@# for file in        - Iterates over all the E2E test suite files.
	@#     ginkgo outline - Exports the Ginkgo DSL outline for a file.
	@#     awk            - Cherry-picks only the Ginkgo DSL nodes (Describe, It, By, etc.) and respective descriptions.
	@#     tail           - Drops the outline header.
	@#     sed            - Adjusts the scenarios to the Gherkin syntax.

	@for file in $(shell ls $(E2E_TESTS_DIR)/*_test.go | grep -v suite_test.go | grep -v utils_test.go) ; do \
		$(GINKGO_CMD) outline --format indent $$file  | \
			awk -F "," '{print $$1" "$$2}' | \
			tail -n +2 | \
			sed -r 's/(By|Context|Describe|It) (Given|When|Then|And|Describe)/\2/' ; \
		done


tools-install: istioctl-install kustomize-install modulectl-install ocm-install

##@ E2E Tests

create-clusters: tools-install ## Create test clusters.
	@echo "::group::Creating test clusters"
	@export PATH=$(LOCALBIN):$$PATH # Add LOCALBIN to PATH
	@$(SCRIPTS_DIR)/create_test_clusters.sh --k8s-version $(K8S_VERSION) --cert-manager-version $(CERT-MANAGER-VERSION)
	@$(SCRIPTS_DIR)/setup_cluster_context.sh
	@echo "::endgroup::"


deploy-klm: ## Deploy KLM in the KCP test cluster.
	echo "::group::Deploying KLM"
	@export PATH=$(LOCALBIN):$$PATH # Add LOCALBIN to PATH
	#
	# The oci-registry-host patch is required for all e2e tests except for: ['oci-reg-cred-secret', 'module-transferred-to-another-oci-registry']
	@echo "Kustomize oci-registry-host"
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	@kustomize edit add patch --path patches/oci_registry_host.yaml --kind Deployment
	@popd > /dev/null
	#
	@if [ -z $$GITHUB_ACTIONS ]; then
		$(SCRIPTS_DIR)/deploy_klm_from_sources.sh
	else
		$(SCRIPTS_DIR)/deploy_klm_from_registry.sh --image-registry $${KLM_IMAGE_REPO} --image-tag $${KLM_VERSION_TAG}
	fi
	@echo "::endgroup::"
	#
	@echo "::group::Patching KCP metrics endpoint"
	@$(SCRIPTS_DIR)/patch_kcp_metrics_endpoint.sh
	@echo "::endgroup::"


test-setup: ## Set up test-specific resources.
	@echo "::group::Setting up test-specific resources"
	@export PATH=$(LOCALBIN):$$PATH # Add LOCALBIN to PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	#
	yq eval '.images[0].newTag = "$(OLDER_VERSION_FOR_MANDATORY_MODULE)"' -i config/manager/deployment/kustomization.yaml
	make build-manifests
	yq eval '(. | select(.kind == "Deployment") | .metadata.name) = "$(MODULE_DEPLOYMENT_NAME_IN_OLDER_VERSION)"' -i template-operator.yaml
	$(SCRIPTS_DIR)/deploy_moduletemplate.sh "$(MODULE_NAME)" "$(OLDER_VERSION_FOR_MANDATORY_MODULE)" "$(DEPLOYABLE_MODULE_VERSION)" true true false false
	kubectl apply -f template.yaml
	rm template.yaml
	#
	$(SCRIPTS_DIR)/deploy_mandatory_modulereleasemeta.sh "$(MODULE_NAME)" "$(OLDER_VERSION_FOR_MANDATORY_MODULE)"
	#
	@popd > /dev/null
	@echo "::endgroup::"


test-run: ginkgo-install ## Run E2E test for mandatory module installation and deletion.
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"
	#
	@echo "::group::E2E test"
	@echo "K8S VERSION: $(K8S_VERSION)"
	@echo "CERT-MANAGER VERSION: $(CERT-MANAGER-VERSION)"
	@echo "KUSTOMIZE VERSION: $(KUSTOMIZE_VERSION)"
	@echo "MODULECTL VERSION: $(MODULECTL_VERSION)"
	@echo "OCM VERSION: $(OCM_VERSION)"
	@echo "GINKGO VERSION: $(GINKGO_VERSION)"
	@export PATH=$(LOCALBIN):$$PATH # Add LOCALBIN to PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e
	@$(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "Mandatory Module Metrics"
	status=$$?
	set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}


test: create-clusters deploy-klm test-setup test-run ## Run all E2E tests for mandatory module installation and deletion.


teardown:
	@echo "::group::Shutting down local clusters..."
	@export PATH=$(LOCALBIN):$$PATH # Add LOCALBIN to PATH
	@$(SCRIPTS_DIR)/clusters_cleanup.sh
	@echo "::endgroup::"
