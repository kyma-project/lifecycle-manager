
# Image URL to use all building/pushing image targets
APP_NAME = lifecycle-manager
IMG_REPO := $(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)
IMG_NAME := $(IMG_REPO)/$(APP_NAME)
IMG := $(IMG_NAME):$(DOCKER_TAG)
BUILD_VERSION := from_makefile

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = $(shell yq e '.envtest_k8s' ./versions.yaml)
ENVTEST_VERSION = $(shell yq e '.envtest' ./versions.yaml)

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

# Go command with FIPS140 Module enabled
GO := GOFIPS140=v1.0.0 go

.PHONY: all
all: build

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

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=controller-manager crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases output:rbac:dir=config/rbac/common

.PHONY: test-crd
test-crd: controller-gen ## Generate crd for test
	$(CONTROLLER_GEN) crd paths="./config/samples/component-integration-installed/crd/..." output:crd:artifacts:config=config/samples/component-integration-installed/crd

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: envtest-dir
envtest-dir:
	echo "$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"

.PHONY: test
test: unittest-api unittest-klm manifests test-crd generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO) test `go list ./tests/integration/...` -ginkgo.flake-attempts 10

.PHONY: unittest-klm
unittest-klm: ## Run the unit test suite.
	$(GO) test `go list ./... | grep -v /tests/` -coverprofile cover.out -coverpkg=./...

.PHONY: unittest-api
unittest-api: ## Run the unit test suite.
	$(GO) test -coverprofile api-cover.out ./api/...

.PHONY: dry-run-control-plane
dry-run-control-plane: kustomize manifests
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	mkdir -p dry-run
	$(KUSTOMIZE) build config/control-plane > dry-run/manifests.yaml

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	$(GO) build -ldflags="-X 'main.buildVersion=${BUILD_VERSION}'" -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
ifneq (,$(GCR_DOCKER_PASSWORD))
	docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
endif
	docker push ${IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: local-deploy-with-watcher
local-deploy-with-watcher: generate kustomize ## Deploy the controller locally with the watcher component using cert-manager for certificate management.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/watcher_local_test | kubectl apply -f -

.PHONY: local-deploy-with-watcher-gcm
local-deploy-with-watcher-gcm: generate kustomize ## Deploy the controller locally with the watcher component using Gardener's cert-manager for certificate management.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/watcher_local_test_gcm | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/control-plane | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries

KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANG_CI_LINT = $(LOCALBIN)/golangci-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v$(shell yq e '.kustomize' ./versions.yaml)
CONTROLLER_TOOLS_VERSION ?= v$(shell yq e '.controllerTools' ./versions.yaml)
GOLANG_CI_LINT_VERSION ?= v$(shell yq e '.golangciLint' ./versions.yaml)

## Tool Install Targets

.PHONY: install-golangci-lint
install-golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANG_CI_LINT_VERSION)

## Tool Execution Targets

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-$(ENVTEST_VERSION)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: lint
lint: install-golangci-lint ## Run golangci-lint against code.
	$(LOCALBIN)/golangci-lint run --verbose -c .golangci.yaml
	pushd api; $(LOCALBIN)/golangci-lint run --verbose -c ../.golangci.yaml; popd
	pushd maintenancewindows; $(LOCALBIN)/golangci-lint run --verbose -c ../.golangci.yaml; popd

.PHONY: lint-yaml
lint-yaml: ## Run yamllint against repository. Assumes yamllint is installed. Install via 'brew install yamllint' or 'pip install yamllint'.
	yamllint -c .yamllint.yml --no-warnings  .
