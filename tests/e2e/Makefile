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

# FIPS140 Module version
FIPS140_MODULE_VERSION := v1.0.0

set-fips:
	export GOFIPS140=$(FIPS140_MODULE_VERSION)
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
	GOBIN=$(LOCALBIN) GOFIPS140=$(FIPS140_MODULE_VERSION) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

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

##@ E2E Tests
GO_TEST := GOFIPS140=$(FIPS140_MODULE_VERSION) go test -timeout 20m -ginkgo.v -ginkgo.focus

kyma-deprovision-with-foreground-propagation:
	$(GO_TEST) "KCP Kyma CR Deprovision With Foreground Propagation After SKR Cluster Removal"

kyma-deprovision-with-background-propagation:
	$(GO_TEST) "KCP Kyma CR Deprovision With Background Propagation After SKR Cluster Removal"

kyma-metrics:
	$(GO_TEST) "Manage Module Metrics"

mandatory-module-metrics:
	$(GO_TEST) "Mandatory Module Metrics"

watcher-enqueue:
	$(GO_TEST) "Enqueue Event from Watcher"

module-status-decoupling-with-statefulset:
	$(GO_TEST) "Module Status Decoupling With StatefulSet"

module-status-decoupling-with-deployment:
	$(GO_TEST) "Module Status Decoupling With Deployment"

module-without-default-cr:
	$(GO_TEST) "Module Without Default CR"

module-consistency:
	$(GO_TEST) "Module Keep Consistent After Deploy"

mandatory-module:
	$(GO_TEST) "Mandatory Module Installation and Deletion"

non-blocking-deletion:
	$(GO_TEST) "Non Blocking Kyma Module Deletion"

skip-manifest-reconciliation:
	$(GO_TEST) "Manifest Skip Reconciliation Label"

modulereleasemeta-upgrade-under-deletion:
	$(GO_TEST) "Kyma Module with ModuleReleaseMeta Upgrade Under Deletion"

unmanage-module:
	$(GO_TEST) "Unmanaging Kyma Module"

purge-controller:
	$(GO_TEST) "Purge Controller"

purge-metrics:
	$(GO_TEST) "Purge Metrics"

module-upgrade-channel-switch:
	$(GO_TEST) "Module Upgrade By Channel Switch"

modulereleasemeta-module-upgrade-new-version:
	$(GO_TEST) "Module with ModuleReleaseMeta Upgrade By New Version"

legacy-istio-gateway-secret-rotation:
	$(GO_TEST) "Legacy Istio Gateway Secret Rotation With Cert-Manager"

legacy-istio-gateway-secret-rotation-gcm:
	$(GO_TEST) "Legacy Istio Gateway Secret Rotation With GCM"

self-signed-certificate-rotation:
	$(GO_TEST) "Self Signed Certificate Rotation"

misconfigured-kyma-secret:
	$(GO_TEST) "Misconfigured Kyma Secret"

rbac-privileges:
	$(GO_TEST) "RBAC Privileges"

oci-reg-cred-secret:
	$(GO_TEST) "OCI Registry Credentials Secret"

modulereleasemeta-with-obsolete-moduletemplate:
	$(GO_TEST) "ModuleReleaseMeta With Obsolete ModuleTemplate"

modulereleasemeta-watch-trigger:
	$(GO_TEST) "ModuleReleaseMeta Watch Trigger"

modulereleasemeta-sync:
	$(GO_TEST) "ModuleReleaseMeta Sync"

module-status-on-skr-connection-lost:
	$(GO_TEST) "KCP Kyma Module status on SKR connection lost"

modulereleasemeta-not-allowed-installation:
	$(GO_TEST) "ModuleReleaseMeta Not Allowed Installation"

labelling:
	$(GO_TEST) "Labelling SKR resources"

maintenance-windows:
	$(GO_TEST) "Maintenance Windows - Wait for Maintenance Window"

maintenance-windows-initial-installation:
	$(GO_TEST) "Maintenance Windows - No Wait for Maintenance Window on Initial Installation"

maintenance-windows-skip:
	$(GO_TEST) "Maintenance Windows - No Wait for Maintenance Widnow on Skip"

watcher-zero-downtime:
	$(GO_TEST) "Watcher Zero Downtime"

module-deletion-blocking-with-multiple-crs-ignore-policy:
	$(GO_TEST) "Blocking Module Deletion With Multiple Module CRs with Ignore Policy"

module-deletion-blocking-with-multiple-crs-create-and-delete-policy:
	$(GO_TEST) "Blocking Module Deletion With Multiple Module CRs with CreateAndDelete Policy"

module-deletion-blocking-with-default-cr-only:
	$(GO_TEST) "Module Deletion With Only Default CR with CreateAndDelete Policy"

module-deletion-blocking-with-default-cr-with-finalizer:
	$(GO_TEST) "Blocking Module Deletion With Default CR With Finalizer with CreateAndDelete Policy"

module-deletion-blocking-with-cross-namespace-ignore-policy:
	$(GO_TEST) "Blocking Module Deletion With Module CRs in Different Namespaces with Ignore Policy"

module-deletion-blocking-with-cross-namespace-create-and-delete-policy:
	$(GO_TEST) "Blocking Module Deletion With Module CRs in Different Namespaces with CreateAndDelete Policy"

module-transferred-to-another-oci-registry:
	$(GO_TEST) "Module Transferred to Another OCI Registry"

fips-mode-metric:
	$(GO_TEST) "FIPS Mode metric"

simulate-skr-secret-rotation-evict-client-cache:
	$(GO_TEST) "SKR client cache get evicted due to connection error caused by secret rotation"

skr-image-pull-secret-sync:
	$(GO_TEST) "SKR Image Pull Secret Sync"

module-api-upgrade-under-blocked-deletion:
	$(GO_TEST) "Module API Upgrade Under Blocked Deletion"
