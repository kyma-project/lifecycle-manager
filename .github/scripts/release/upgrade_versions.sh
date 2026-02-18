#!/usr/bin/env bash

# This script bumps version references in sec-scanners-config.yaml
# to match the release version being created.
# Note: kustomization.yaml files keep 'latest' and are updated dynamically
# via IMG variable during manifest generation.
#
# Required environment variables:
#   IMG_VERSION: The version to set (e.g., "1.3.9")

set -e

if [ -z "${IMG_VERSION}" ]; then
  echo "Error: IMG_VERSION environment variable is not set"
  exit 1
fi

echo "Upgrading version references to ${IMG_VERSION}"

# Update sec-scanners-config.yaml
# Replace the first BDBA entry with the new version
if [ -f "sec-scanners-config.yaml" ]; then
  echo "Updating sec-scanners-config.yaml..."
  # Get the image name from the first bdba entry
  IMAGE_NAME=$(yq eval '.bdba[0]' sec-scanners-config.yaml | cut -d: -f1)
  # Update the first entry (non-latest) with new version
  yq eval ".bdba[0] = \"${IMAGE_NAME}:${IMG_VERSION}\"" -i sec-scanners-config.yaml
  echo "✓ Updated sec-scanners-config.yaml"
else
  echo "Warning: sec-scanners-config.yaml not found"
fi

echo "Version upgrade complete!"
