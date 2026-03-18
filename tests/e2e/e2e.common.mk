# e2e.common.mk – shared infrastructure for all E2E test makefiles.
#
# Include this file at the top of each test-specific .mk file:
#   include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk
#
# Each test file MUST define:
#   - module-setup : deploy test-specific module templates and metadata
#   - klm-patch    : apply any test-specific patches to lifecycle-manager manifests (if needed)
#   - test-run     : run the Ginkgo test with the correct focus string
#   - test         : top-level target that chains all steps together

##@ Sanity checks

ifeq ($(filter oneshell,$(.FEATURES)),)
  $(error .ONESHELL is not supported by this make version (need >= 3.82). \
    macOS ships make 3.81 - install a newer version via Homebrew: brew install make)
endif


##@ Shell configuration

.ONESHELL:
SHELL := bash
.SHELLFLAGS = -o pipefail -ec


##@ Go setup

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set).
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Go command with FIPS140 module enabled.
GO := GOFIPS140=v1.0.0 go


##@ Important directories

LIFECYCLE_MANAGER_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../)
E2E_TESTS_DIR         := $(realpath $(LIFECYCLE_MANAGER_DIR)/tests/e2e/)
SCRIPTS_DIR           := $(realpath $(LIFECYCLE_MANAGER_DIR)/scripts/tests/)
INSTALL_SCRIPTS_DIR   := $(realpath $(LIFECYCLE_MANAGER_DIR)/scripts/install/)
TEMPLATE_OPERATOR_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/../template-operator/)
LOCALBIN              ?= $(E2E_TESTS_DIR)/bin


##@ Tool versions
# Note: no "v" prefix — the install scripts expect bare version numbers.

GINKGO_VERSION        ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_ginkgo_version.sh)
K8S_VERSION           ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_k8s_version.sh)
CERT_MANAGER_VERSION  ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_cert_manager_version.sh)
ISTIOCTL_VERSION      ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_istioctl_version.sh)
KUSTOMIZE_VERSION     ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_kustomize_version.sh)
MODULECTL_VERSION     ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_modulectl_version.sh)
OCM_VERSION           ?= $(shell $(INSTALL_SCRIPTS_DIR)/required_ocm_version.sh)

# Ginkgo binary path.
GINKGO_CMD ?= $(LOCALBIN)/ginkgo


##@ General targets

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
	      /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
	      /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

$(LOCALBIN):
	@mkdir -p $(LOCALBIN)


##@ Tool installation

# Each install target checks for the binary first and skips installation if it already exists.
# This makes repeated local runs faster and avoids overwriting tools managed outside LOCALBIN.

.PHONY: ginkgo-install
ginkgo-install: $(LOCALBIN) ## Install ginkgo into LOCALBIN (skipped if already present).
	@echo "::group::Install ginkgo"
	@if [ -f $(LOCALBIN)/ginkgo ]; then \
		echo "ginkgo already installed, skipping"; \
	else \
		pushd $(LOCALBIN) > /dev/null && \
		$(INSTALL_SCRIPTS_DIR)/ginkgo_install.sh $(GINKGO_VERSION) && \
		popd > /dev/null; \
	fi
	@echo "::endgroup::"

.PHONY: kustomize-install
kustomize-install: $(LOCALBIN) ## Install kustomize into LOCALBIN (skipped if already present).
	@echo "::group::Install kustomize"
	@if [ -f $(LOCALBIN)/kustomize ]; then \
		echo "kustomize already installed, skipping"; \
	else \
		pushd $(LOCALBIN) > /dev/null && \
		$(INSTALL_SCRIPTS_DIR)/kustomize_install.sh $(KUSTOMIZE_VERSION) && \
		popd > /dev/null; \
	fi
	@echo "::endgroup::"

.PHONY: modulectl-install
modulectl-install: $(LOCALBIN) ## Install modulectl into LOCALBIN (skipped if already present).
	@echo "::group::Install modulectl"
	@if [ -f $(LOCALBIN)/modulectl ]; then \
		echo "modulectl already installed, skipping"; \
	else \
		pushd $(LOCALBIN) > /dev/null && \
		$(INSTALL_SCRIPTS_DIR)/modulectl_install.sh $(MODULECTL_VERSION) && \
		popd > /dev/null; \
	fi
	@echo "::endgroup::"

.PHONY: istioctl-install
istioctl-install: $(LOCALBIN) ## Install istioctl into LOCALBIN (skipped if already present).
	@echo "::group::Install istioctl"
	@if [ -f $(LOCALBIN)/istioctl ]; then \
		echo "istioctl already installed, skipping"; \
	else \
		pushd $(LOCALBIN) > /dev/null && \
		$(INSTALL_SCRIPTS_DIR)/istioctl_install.sh $(ISTIOCTL_VERSION) && \
		popd > /dev/null; \
	fi
	@echo "::endgroup::"

.PHONY: ocm-install
ocm-install: $(LOCALBIN) ## Install ocm CLI into LOCALBIN (skipped if already present).
	@echo "::group::Install ocm"
	@if [ -f $(LOCALBIN)/ocm ]; then \
		echo "ocm already installed, skipping"; \
	else \
		pushd $(LOCALBIN) > /dev/null && \
		$(INSTALL_SCRIPTS_DIR)/ocm_install.sh $(OCM_VERSION) && \
		popd > /dev/null; \
	fi
	@echo "::endgroup::"

.PHONY: tools-install
tools-install: istioctl-install kustomize-install modulectl-install ocm-install ginkgo-install


##@ Cluster lifecycle

.PHONY: create-clusters
create-clusters: tools-install ## Create KCP and SKR test clusters.
	@echo "::group::Creating test clusters"
	@export PATH=$(LOCALBIN):$$PATH
	@$(SCRIPTS_DIR)/create_test_clusters.sh --k8s-version $(K8S_VERSION) --cert-manager-version $(CERT_MANAGER_VERSION)
	@$(SCRIPTS_DIR)/setup_cluster_context.sh
	@echo "::endgroup::"


.PHONY: deploy-klm
deploy-klm: ## Deploy KLM into the KCP test cluster.
	@echo "::group::Deploying KLM"
	@export PATH=$(LOCALBIN):$$PATH
	@echo "Applying kustomize oci-registry-host patch"
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	@kustomize edit add patch --path patches/oci_registry_host.yaml --kind Deployment
	@popd > /dev/null
	@if [ -z "$$GITHUB_ACTIONS" ]; then
		$(SCRIPTS_DIR)/deploy_klm_from_sources.sh
	else
		$(SCRIPTS_DIR)/deploy_klm_from_registry.sh --image-registry $${KLM_IMAGE_REPO} --image-tag $${KLM_VERSION_TAG}
	fi
	@echo "::endgroup::"
	@echo "::group::Patching KCP metrics endpoint"
	@$(SCRIPTS_DIR)/patch_kcp_metrics_endpoint.sh
	@echo "::endgroup::"

.PHONY: teardown
teardown: ## Delete KCP and SKR test clusters.
	@echo "::group::Shutting down local clusters"
	@export PATH=$(LOCALBIN):$$PATH
	@$(SCRIPTS_DIR)/clusters_cleanup.sh
	@echo "::endgroup::"


##@ Test helpers

.PHONY: log-tool-versions
log-tool-versions: ## Print the versions of all tools used in the test run.
	@echo "::group::Tool versions"
	@echo "K8S VERSION:          $(K8S_VERSION)"
	@echo "CERT-MANAGER VERSION: $(CERT_MANAGER_VERSION)"
	@echo "ISTIOCTL VERSION:     $(ISTIOCTL_VERSION)"
	@echo "KUSTOMIZE VERSION:    $(KUSTOMIZE_VERSION)"
	@echo "MODULECTL VERSION:    $(MODULECTL_VERSION)"
	@echo "OCM VERSION:          $(OCM_VERSION)"
	@echo "GINKGO VERSION:       $(GINKGO_VERSION)"
	@echo "::endgroup::"
