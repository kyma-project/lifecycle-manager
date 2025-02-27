.DEFAULT_GOAL := test

# Mark all the targets in the file as PHONY ones.
# Effectively, all the targets passed to the make during the invocation are
# passed through the MAKECMDGOALS special variable. So, the instruction below
# defines the targets mentioned above as phony ones.
#
# The test target is explicitly listed because it is subject to omission.
.PHONY: test $(MAKECMDGOALS)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

## Location to install local dependencies to.
LOCALBIN ?= $(shell pwd)/../../bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
.ONESHELL:
SHELL = /bin/bash
.SHELLFLAGS = -o pipefail -ec

# Ginkgo binary metadata.
GINKGO ?= $(LOCALBIN)/ginkgo
GINKGO_VERSION ?= v2.12.0

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

ginkgo: $(GINKGO) ## Download local ginkgo binary if necessary.
$(GINKGO): $(LOCALBIN)
	@test -s $(GINKGO) && $(GINKGO) version | grep -q $(GINKGO_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

e2e-coverage: ginkgo ## Generate the effective Acceptance Criteria for all the test suites.
	@# for file in        - Iterates over all the E2E test suite files.
	@#     ginkgo outline - Exports the Ginkgo DSL outline for a file.
	@#     awk            - Cherry-picks only the Ginkgo DSL nodes (Describe, It, By, etc.) and respective descriptions.
	@#     tail           - Drops the outline header.
	@#     sed            - Adjusts the scenarios to the Gherkin syntax.

	@for file in $(shell ls *_test.go | grep -v suite_test.go | grep -v utils_test.go) ; do \
        $(GINKGO) outline --format indent $$file  | \
			awk -F "," '{print $$1" "$$2}' | \
			tail -n +2 | \
			sed -r 's/(By|Context|Describe|It) (Given|When|Then|And|Describe)/\2/' ; \
    done

## Utility

LIFECYCLE_MANAGER_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../)
TEMPLATE_OPERATOR_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/../template-operator/)
SCRIPTS_DIR := $(realpath $(LIFECYCLE_MANAGER_DIR)/scripts/tests/)

define setup-clusters
	echo "::group::Setting up test clusters"
	@$(SCRIPTS_DIR)/create_test_clusters.sh --k8s-version 1.30.3 --cert-manager-version 1.13.3
	echo "::endgroup::"

	echo "::group::Setting up cluster context"
	@$(SCRIPTS_DIR)/setup_cluster_context.sh
	echo "::endgroup::"
endef

define setup-klm
	echo "::group::Setting up KLM"
	@if [ -z $$GITHUB_ACTIONS ]; then
		@$(SCRIPTS_DIR)/deploy_klm_from_sources.sh
	else
		@$(SCRIPTS_DIR)/deploy_klm_from_registry.sh --image-registry $(IMAGE_REPO) --image-tag $(IMAGE_TAG)
	fi
	echo "::endgroup::"

	echo "::group::Patching KCP metrics endpoint"
	@$(SCRIPTS_DIR)/patch_kcp_metrics_endpoint.sh
	echo "::endgroup::"
endef

define setup-kubeconfig-var
	echo "::group::Setting kubeconfig variables"
	export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	echo "::endgroup::"
endef

##@ E2E Tests
mandatory-module-setup:
	@$(setup-clusters)
	@echo "::group::Test-specific kustomize patch"
	@echo "::endgroup::"
	@$(setup-klm)

	@echo "::group::Test-specific module metadata setup"
	@pushd $(TEMPLATE_OPERATOR_DIR)
	@$(SCRIPTS_DIR)/setup_https_server.sh .
	@$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh template-operator 1.1.0-smoke-test template-operator-v1-controller-manager true true
	@$(SCRIPTS_DIR)/deploy_moduletemplate_e2e.sh template-operator 2.4.1-smoke-test template-operator-v2-controller-manager true true false
	@cp ./template.yaml $(LIFECYCLE_MANAGER_DIR)/tests/e2e/mandatory_template_v2.yaml
	@popd
	@echo "::endgroup::"

mandatory-module:
	@$(setup-kubeconfig-var)
	@echo "::group::E2E test"
	@go test -timeout 20m -ginkgo.v -ginkgo.focus "Mandatory Module Installation and Deletion"
	@echo "::endgroup::"

teardown:
	# TODO
