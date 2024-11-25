#!/bin/bash

# TODO: where to extract the latest version of the tools from?

# Source: https://semver.org/
# Using a simplified version of semantic versioning regex pattern, which is bash compatible
SEM_VER_REGEX="^([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"

# Set default values for variables
KUBECTL_VERSION_DEFAULT="1.31.3"
GO_VERSION_DEFAULT="1.23.3"
K3D_VERSION_DEFAULT="5.6.0"
DOCKER_VERSION_DEFAULT="27.3.1"

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

# Parse command line arguments
while [[ "$#" -gt 0 ]]; do
  case $1 in
    --kubectl-version)
      KUBECTL_VERSION="$2"
      if [[ ! $KUBECTL_VERSION =~ $SEM_VER_REGEX ]]; then
        echo "Invalid kubectl version: $KUBECTL_VERSION"
        exit 2
      fi

      [[ $(version_comparator "$KUBECTL_VERSION" "$KUBECTL_VERSION_DEFAULT") -eq 1 ]] \
        && print_warning "kubectl" "$KUBECTL_VERSION_DEFAULT" \
        || echo "kubectl version is up to date"

      shift 2
      ;;
    --go-version)
      GO_VERSION="$2"
      if [[ ! $GO_VERSION =~ $SEM_VER_REGEX ]]; then
        echo "Invalid Go version: $GO_VERSION"
        exit 2
      fi

      [[ $(version_comparator "$GO_VERSION" "$GO_VERSION_DEFAULT") -eq 1 ]] \
        && print_warning "Go" "$GO_VERSION_DEFAULT" \
        || echo "kubectl version is up to date"

      shift 2
      ;;
    --k3d-version)
      K3D_VERSION="$2"
      if [[ ! $K3D_VERSION =~ $SEM_VER_REGEX ]]; then
        echo "Invalid k3d version: $K3D_VERSION"
        exit 2
      fi

      [[ $(version_comparator "$K3D_VERSION" "$K3D_VERSION_DEFAULT") -eq 1 ]] \
        && print_warning "k3d" "$K3D_VERSION_DEFAULT" \
        || echo "kubectl version is up to date"

      shift 2
      ;;
    --docker-version)
      DOCKER_VERSION="$2"
      if [[ ! $DOCKER_VERSION =~ $SEM_VER_REGEX ]]; then
        echo "Invalid Docker version: $DOCKER_VERSION"
        exit 2
      fi

      [[ $(version_comparator "$DOCKER_VERSION" "$DOCKER_VERSION_DEFAULT") -eq 1 ]] \
        && print_warning "docker" "$DOCKER_VERSION_DEFAULT" \
        || echo "kubectl version is up to date"

      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      echo "Example usage:"
      echo "  ./version.sh --kubectl-version kubectl_version --go-version go_version --k3d-version k3d_version --docker-version docker_version"
      echo "  ./version.sh --kubectl-version kubectl_version --go-version go_version"
      echo "  ./version.sh"
      exit 1
      ;;
  esac
done

# Use default values if not provided
KUBECTL_VERSION=${KUBECTL_VERSION:-$KUBECTL_VERSION_DEFAULT}
GO_VERSION=${GO_VERSION:-$GO_VERSION_DEFAULT}
K3D_VERSION=${K3D_VERSION:-$K3D_VERSION_DEFAULT}
DOCKER_VERSION=${DOCKER_VERSION:-$DOCKER_VERSION_DEFAULT}

# Print the arguments
echo "Using the following versions:"
echo "kubectl: $KUBECTL_VERSION"
echo "Go: $GO_VERSION"
echo "k3d: $K3D_VERSION"
echo "Docker: $DOCKER_VERSION"

# TODO: Export the versions or set corresponding environment variables
# Or exit with error code 1 if the desired versions are not present
