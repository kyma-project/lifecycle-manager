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
	@# for file in        - Iterates over all the E2E test suite files.
	@#     ginkgo outline - Exports the Ginkgo DSL outline for a file.
	@#     awk            - Cherry-picks only the Ginkgo DSL nodes (Describe, It, By, etc.) and respective descriptions.
	@#     tail           - Drops the outline header.
	@#     sed            - Adjusts the scenarios to the Gherkin syntax.

	@for file in $(shell ls *_test.go | grep -v suite_test.go) ; do \
        $(GINKGO) outline --format indent $$file  | \
			awk -F "," '{print $$1" "$$2}' | \
			tail -n +2 | \
			sed -r 's/(By|Context|Describe|It) (Given|When|Then|And|Describe)/\2/' ; \
    done

##@ E2E Tests

test: watcher-enqueue kyma-deletion module-status-propagation

kyma-deletion:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "KCP Kyma CR Deletion"

kyma-metrics:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Kyma Metrics"

watcher-enqueue:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Enqueue Event from Watcher"

module-status-propagation:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Warning Status Propagation"

module-without-default-cr:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Module Without Default CR"

non-blocking-deletion:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Non Blocking Kyma Module Deletion"

purge-controller:
	go test  -timeout 20m -ginkgo.v -ginkgo.focus "Purge Controller"

purge-metrics:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Purge Metrics"

module-upgrade:
	go test -timeout 20m -ginkgo.v -ginkgo.focus "Module Upgrade"

certificate-rotation:
	go test -timeout 20m -ginkgo.v ginkgo.focus "Certificate Rotation"
