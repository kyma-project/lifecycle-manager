#!/usr/bin/env bash

# This script bumps version references in a security scanner config file.
# It adds the new version to the beginning of the BDBA array, keeps the current version as the second entry, 
# and ensures "latest" is included as the third entry.
#
# Usage: ./upgrade_versions.sh <version> <image-name> <sec-scanners-file>
#   version: The version to set (e.g., "1.3.9")
#   image-name: The base image name (e.g., "europe-docker.pkg.dev/kyma-project/prod/lifecycle-manager")
#   sec-scanners-file: The config file to update (e.g., "sec-scanners-config.yaml")

set -e

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
  echo "Error: Version, image-name, and sec-scanners-file arguments are required"
  echo "Usage: $0 <version> <image-name> <sec-scanners-file>"
  exit 1
fi

IMG_VERSION="$1"
IMAGE_NAME="$2"
SEC_SCANNERS_FILE="$3"
echo "Adding ${IMG_VERSION} to ${SEC_SCANNERS_FILE}"

# Reconstruct the BDBA array with new version, current version, and latest
CURRENT_VERSION=$(yq eval '.bdba[0]' "${SEC_SCANNERS_FILE}")
yq eval ".bdba = [\"${IMAGE_NAME}:${IMG_VERSION}\", \"${CURRENT_VERSION}\", \"${IMAGE_NAME}:latest\"]" -i "${SEC_SCANNERS_FILE}"

echo "Updated BDBA array in ${SEC_SCANNERS_FILE}:"
yq eval '.bdba' "${SEC_SCANNERS_FILE}"
