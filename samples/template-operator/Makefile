# Module Name used for bundling the OCI Image and later on for referencing in the Kyma Modules
MODULE_NAME ?= template
# Semantic Module Version used for identifying the build
MODULE_VERSION ?= 0.0.4
# Module Registry used for pushing the image
MODULE_REGISTRY_PORT ?= 50092
MODULE_REGISTRY ?= op-kcp-registry.localhost:$(MODULE_REGISTRY_PORT)/unsigned
# Desired Channel of the Generated Module Template
MODULE_TEMPLATE_CHANNEL ?= stable

# Credentials used for authenticating into the module registry
# see `kyma alpha mod create --help for more info`
# MODULE_CREDENTIALS ?= testuser:testpw

# Image URL to use all building/pushing image targets
IMG_REGISTRY_PORT ?= $(MODULE_REGISTRY_PORT)
IMG_REGISTRY ?= op-skr-registry.localhost:$(IMG_REGISTRY_PORT)/unsigned/operator-images
IMG ?= $(IMG_REGISTRY)/$(MODULE_NAME)-operator:$(MODULE_VERSION)

COMPONENT_CLI_VERSION ?= latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# This will change the flags of the `kyma alpha module create` command in case we spot credentials
# Otherwise we will assume http-based local registries without authentication (e.g. for k3d)
ifeq (,$(MODULE_CREDENTIALS))
MODULE_CREATION_FLAGS=--registry $(MODULE_REGISTRY) -w --insecure
else
MODULE_CREATION_FLAGS=--registry $(MODULE_REGISTRY) -w -c $(MODULE_CREDENTIALS)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
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

.PHONY: module-image
module-image: operator/docker-build operator/docker-push ## Build the Module Image and push it to a registry defined in IMG_REGISTRY
	echo "built and pushed module image $(IMG)"

.PHONY: module-build
module-build: kyma module-operator-chart module-default ## Build the Module and push it to a registry defined in MODULE_REGISTRY
	$(KYMA) alpha create module kyma.project.io/module/$(MODULE_NAME) $(MODULE_VERSION) . $(MODULE_CREATION_FLAGS)

.PHONY: module-template-push
module-template-push: ## Pushes the ModuleTemplate referencing the Image on MODULE_REGISTRY
	kubectl apply -f template.yaml

.PHONY: module-default
module-default:
	cp operator/config/samples/* default.yaml

##@ Tools

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

########## Kyma CLI ###########
KYMA_STABILITY ?= unstable

KYMA ?= $(LOCALBIN)/kyma-$(KYMA_STABILITY)
kyma: $(KYMA) ## Download kyma locally if necessary.
$(KYMA):
	test -f $@ || curl -# -Lo $(KYMA) https://storage.googleapis.com/kyma-cli-$(KYMA_STABILITY)/kyma-darwin 
	chmod 0100 $(KYMA)

########## Kustomize ###########
KUSTOMIZE_VERSION ?= v4.5.6
KUSTOMIZE ?= $(LOCALBIN)/kustomize
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download & Build kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION)

########## Grafana Dashboard ###########
.PHONY: grafana-dashboard
grafana-dashboard: ## Generating Grafana manifests to visualize controller status.
	cd operator && kubebuilder edit --plugins grafana.kubebuilder.io/v1-alpha
