#!/usr/bin/env bash
# Generates ApplyConfiguration helper types for cert-management API types into
# api/applyconfigurations/cert/gardener/. Re-run this script whenever cert-management is upgraded.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OUTPUT_DIR="${REPO_ROOT}/api/applyconfigurations/cert/gardener"
OUTPUT_PKG="github.com/kyma-project/lifecycle-manager/api/applyconfigurations/cert/gardener"
INPUT_PKG="github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
BOILERPLATE="${SCRIPT_DIR}/boilerplate.go.txt"

# Build the generator from the module cache using the k8s version already
# pinned in go.mod, so no separate tool installation is required.
CODE_GEN_VERSION="$(go list -m -f '{{.Version}}' k8s.io/code-generator 2>/dev/null || echo "")"
if [[ -z "${CODE_GEN_VERSION}" ]]; then
  # code-generator is not a direct dependency; derive the version from client-go
  # which is always kept in sync with it.
  CLIENT_GO_VERSION="$(go list -m -f '{{.Version}}' k8s.io/client-go)"
  CODE_GEN_VERSION="${CLIENT_GO_VERSION}"
fi

echo "Using k8s.io/code-generator@${CODE_GEN_VERSION}"

APPLYCONFIGURATION_GEN_BIN="$(go env GOPATH)/bin/applyconfiguration-gen"
if [[ ! -x "${APPLYCONFIGURATION_GEN_BIN}" ]]; then
  echo "Building applyconfiguration-gen..."
  GOBIN="$(go env GOPATH)/bin" go install "k8s.io/code-generator/cmd/applyconfiguration-gen@${CODE_GEN_VERSION}"
fi

echo "Generating ApplyConfigurations into ${OUTPUT_DIR} ..."
"${APPLYCONFIGURATION_GEN_BIN}" \
  --output-dir "${OUTPUT_DIR}" \
  --output-pkg "${OUTPUT_PKG}" \
  --go-header-file "${BOILERPLATE}" \
  --external-applyconfigurations k8s.io/api/core/v1.SecretReference:k8s.io/client-go/applyconfigurations/core/v1 \
  "${INPUT_PKG}"

echo "Done."
