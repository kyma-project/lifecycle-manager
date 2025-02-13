#!/bin/bash

# Source: https://semver.org/
# Using a simplified version of semantic versioning regex pattern, which is bash compatible
SEM_VER_REGEX="^([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"

# Change to root directory of the project
cd "$(git rev-parse --show-toplevel)"

# Set default values for variables
KUBECTL_VERSION_DEFAULT=$(yq e '.kubectl' ./versions.yaml)
GO_VERSION_DEFAULT=$(yq e '.go' ./versions.yaml)
K3D_VERSION_DEFAULT=$(yq e '.k3d' ./versions.yaml)
DOCKER_VERSION_DEFAULT=$(yq e '.docker' ./versions.yaml)
ISTIOCTL_VERSION_DEFAULT=$(yq e '.istio' ./versions.yaml)

versioning_error=false
# Check if required tools are installed
if ! command -v kubectl &> /dev/null; then
  echo "kubectl is not installed. Please install kubectl."
  versioning_error=true
fi

if ! command -v go &> /dev/null; then
  echo "Go is not installed. Please install Go."
  versioning_error=true
fi

if ! command -v k3d &> /dev/null; then
  echo "k3d is not installed. Please install k3d."
  versioning_error=true
fi

if ! command -v docker &> /dev/null; then
  echo "Docker is not installed. Please install Docker."
  versioning_error=true
fi

if ! command -v istioctl &> /dev/null; then
  echo "istioctl is not installed. Please install istioctl."
  versioning_error=true
fi

if $versioning_error; then
  exit 1
fi

# Versions installed on current system
KUBECTL_VERSION_INSTALLED=$(kubectl version --client | grep -E '[0-9]{1,}.[0-9]{1,}.[0-9]{1,}' | head -n1 | awk '{print $3}' | sed 's/v//')
GO_VERSION_INSTALLED=$(go version | awk '{print $3}' | sed 's/go//')
K3D_VERSION_INSTALLED=$(k3d --version | head -n1 | awk '{print $3}' | sed 's/v//')
DOCKER_VERSION_INSTALLED=$(docker --version | awk '{print $3}' | cut -d',' -f1)
ISTIOCTL_VERSION_INSTALLED=$(istioctl version --short --remote=false | awk '{print $NF}' | sed 's/v//')

# Function to compare two versions
# Returns:
# 0 if the versions are equal
# 1 if the first version is less than the second version
# 2 if the first version is greater than the second version
function version_comparator() {
  if [[ "$1" == "$2" ]]; then
    echo 0
    return
  fi

  local first_version; first_version=$(echo -e "$1\n$2" | sort --version-sort | head -n1)

  if [[ "$first_version" == "$1" ]]; then
    echo 1
    return
  fi
  echo 2
  return
}

function print_warning() {
  echo "[WARNING] Using a version of $1 that is older than the recommended version: $2"
}

# Check for regex patterns with semver (Semantic Versioning)
if [[ ! $KUBECTL_VERSION_INSTALLED =~ $SEM_VER_REGEX ]]; then
  echo "Invalid kubectl version: $KUBECTL_VERSION_INSTALLED"
  exit 2
fi

if [[ ! $GO_VERSION_INSTALLED =~ $SEM_VER_REGEX ]]; then
  echo "Invalid Go version: $GO_VERSION_INSTALLED"
  exit 2
fi

if [[ ! $K3D_VERSION_INSTALLED =~ $SEM_VER_REGEX ]]; then
  echo "Invalid k3d version: $K3D_VERSION_INSTALLED"
  exit 2
fi

if [[ ! $DOCKER_VERSION_INSTALLED =~ $SEM_VER_REGEX ]]; then
  echo "Invalid Docker version: $DOCKER_VERSION_INSTALLED"
  exit 2
fi

if [[ ! $ISTIOCTL_VERSION_INSTALLED =~ $SEM_VER_REGEX ]]; then
  echo "Invalid istioctl version: $ISTIOCTL_VERSION_INSTALLED"
  exit 2
fi

# Check if the installed versions are up to date
[[ $(version_comparator "$KUBECTL_VERSION_INSTALLED" "$KUBECTL_VERSION_DEFAULT") -eq 1 ]] \
  && print_warning "kubectl" "$KUBECTL_VERSION_DEFAULT" \
  || echo "kubectl  version is up to date, using: v$KUBECTL_VERSION_INSTALLED"

[[ $(version_comparator "$GO_VERSION_INSTALLED" "$GO_VERSION_DEFAULT") -eq 1 ]] \
  && print_warning "Go" "$GO_VERSION_DEFAULT" \
  || echo "GoLang   version is up to date, using: go$GO_VERSION_INSTALLED"

[[ $(version_comparator "$K3D_VERSION_INSTALLED" "$K3D_VERSION_DEFAULT") -eq 1 ]] \
  && print_warning "k3d" "$K3D_VERSION_DEFAULT" \
  || echo "k3d      version is up to date, using: v$K3D_VERSION_INSTALLED"

[[ $(version_comparator "$DOCKER_VERSION_INSTALLED" "$DOCKER_VERSION_DEFAULT") -eq 1 ]] \
  && print_warning "docker" "$DOCKER_VERSION_DEFAULT" \
  || echo "docker   version is up to date, using: v$DOCKER_VERSION_INSTALLED"

[[ $(version_comparator "$ISTIOCTL_VERSION_INSTALLED" "$ISTIOCTL_VERSION_DEFAULT") -eq 1 ]] \
  && print_warning "istioctl" "$ISTIOCTL_VERSION_DEFAULT" \
  || echo "istioctl version is up to date, using: v$ISTIOCTL_VERSION_INSTALLED"

# Exit with success
exit 0
