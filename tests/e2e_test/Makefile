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
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

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
	@for file in $(shell ls *_test.go | grep -v suite_test.go) ; do \
        $(GINKGO) outline --format indent $$file  | awk -F "," '{print $$1" "$$2}' | tail -n +2 ; \
    done

##@ E2E Tests

test: watcher-enqueue kyma-deletion module-status-propagation

kyma-deletion: ## Runs the Kyma Deletion E2E Test
	go test -ginkgo.v -ginkgo.focus "KCP Kyma CR Deletion"

kyma-metrics: ## Runs the Kyma Metrics E2E Test
	go test -ginkgo.v -ginkgo.focus "Kyma Metrics"

watcher-enqueue: ## Runs the Watcher E2E Test
	go test -ginkgo.v -ginkgo.focus "Enqueue Event from Watcher"

module-status-propagation: ## Runs the Status Propagation E2E Test
	go test -ginkgo.v -ginkgo.focus "Warning Status Propagation"
