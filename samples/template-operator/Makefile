# Module Name used for bundling the OCI Image and later on for referencing in the Kyma Modules
MODULE_NAME ?= template
# Semantic Module Version used for identifying the build
MODULE_VERSION ?= 0.0.0
# Module Registry used for pushing the image
MODULE_REGISTRY ?= op-kcp-registry:56888/unsigned
# Desired Channel of the Generated Module Template
MODULE_TEMPLATE_CHANNEL ?= stable

# Image URL to use all building/pushing image targets
IMG_REGISTRY ?= op-kcp-registry:56888/operator-images
IMG ?= $(IMG_REGISTRY)/$(MODULE_NAME)-operator:$(MODULE_VERSION)

KYMA_VERSION ?= 2dd54fae46e6d2e66ce0419dfbcf6ad789a64c09
COMPONENT_CLI_VERSION ?= latest
KUSTOMIZE_VERSION ?= v4.5.6

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: module-build

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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: operator/manifests
operator/manifests: ## Call Manifest Generation
	$(MAKE) -C operator/ manifests

.PHONY: operator/docker-build
operator/docker-build:
	IMG=$(IMG) $(MAKE) -C operator/ docker-build
.PHONY: operator/docker-push
operator/docker-push:
	IMG=$(IMG) $(MAKE) -C operator/ docker-push


##@ Module

TEMPLATE_DIR ?= charts/$(MODULE_NAME)-operator
GEN_CHART ?= sh hack/gen-chart.sh
GEN_MODULE_TEMPLATE ?= sh hack/gen-mod-template.sh

.PHONY: module-operator-chart
module-operator-chart: operator/manifests kustomize ## Bundle the Module Operator Chart
	mkdir -p "$(TEMPLATE_DIR)"/templates $(TEMPLATE_DIR)/crds/
	cd operator/config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build operator/config/default -o $(TEMPLATE_DIR)/templates/
	mv $(TEMPLATE_DIR)/templates/apiextensions.k8s.io_v1_customresourcedefinition_* $(TEMPLATE_DIR)/crds
	MODULE_NAME=$(MODULE_NAME) MODULE_VERSION=$(MODULE_VERSION) $(GEN_CHART) > $(TEMPLATE_DIR)/Chart.yaml

.PHONY: module-template-build 
module-template-build: module-build module-default ## Deploy a Module and create a ModuleTemplate from a Descriptor and the Default data
	$(COMPONENT_CLI) ca remote get $(MODULE_REGISTRY) kyma.project.io/module/$(MODULE_NAME) $(MODULE_VERSION) --allow-plain-http > ./mod/remote-component-descriptor.yaml
	DEFAULT_DATA=default.yaml REMOTE_SIGNED_DESCRIPTOR=./mod/remote-component-descriptor.yaml MODULE_NAME=$(MODULE_NAME) OPERATOR_NAME=kyma-operator MODULE_TEMPLATE_CHANNEL=$(MODULE_TEMPLATE_CHANNEL) MODULE_TEMPLATE=template.yaml $(GEN_MODULE_TEMPLATE)

.PHONY: module-image
module-image: operator/docker-build operator/docker-push ## Build the Module Image and push it to a registry defined in IMG_REGISTRY
	echo "built and pushed module image $(IMG)"

.PHONY: module-build
module-build: kyma component-cli module-operator-chart ## Build the Module and push it to a registry defined in MODULE_REGISTRY
	$(KYMA) alpha create module kyma.project.io/module/$(MODULE_NAME) $(MODULE_VERSION) . \
		--registry $(MODULE_REGISTRY) \
		--insecure \
		-w

.PHONY: module-default
module-default:
	if [ ! -d default.yaml ]; then cp operator/config/samples/* default.yaml; fi

##@ Tools

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

########## Kyma CLI ###########
KYMA_CLI_CHECKOUT_PATH = "/tmp/operator/make-build/kyma-cli"
KYMA ?= $(LOCALBIN)/kyma
.PHONY: get-kyma
get-kyma:
	if [ ! -d $(KYMA_CLI_CHECKOUT_PATH) ]; then GOBIN=$(LOCALBIN) git clone https://github.com/kyma-project/cli $(KYMA_CLI_CHECKOUT_PATH) -b main --depth 100; fi
	cd $(KYMA_CLI_CHECKOUT_PATH) && git checkout $(KYMA_VERSION)

.PHONY: kyma
kyma: get-kyma $(KYMA) ## Download & Build Kyma CLI locally if necessary.
$(KYMA): $(LOCALBIN)
	cd $(KYMA_CLI_CHECKOUT_PATH) && make resolve && go build -o $(KYMA) -ldflags '-s -w -X github.com/kyma-project/cli/cmd/kyma/version.Version=$(KYMA_VERSION)' ./cmd 

########## Kustomize ###########
KUSTOMIZE ?= $(LOCALBIN)/kustomize
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download & Build kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION)

########## Component-CLI ###########
COMPONENT_CLI ?= $(LOCALBIN)/component-cli
.PHONY: component-cli
component-cli: $(COMPONENT_CLI) ## Download & Build component-cli locally if necessary.
$(COMPONENT_CLI): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/gardener/component-cli/cmd/component-cli@$(COMPONENT_CLI_VERSION)

########## Grafana Dashboard ###########
.PHONY: grafana-dashboard
grafana-dashboard: ## Generating Grafana manifests to visualize controller status.
	cd operator && kubebuilder edit --plugins grafana.kubebuilder.io/v1-alpha