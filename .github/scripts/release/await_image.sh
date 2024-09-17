#!/usr/bin/env bash

# standard bash error handling
set -o nounset  # treat unset variables as an error and exit immediately.
set -o errexit  # exit immediately when a command fails.
set -E          # must be set if you want the ERR trap
set -o pipefail # prevents errors in a pipeline from being masked

# This script has the following arguments:
#                       - binary image tag - mandatory
#
# ./await_image.sh 1.1.0

# Expected variables:
#             IMAGE_REPO - binary image repository
#             GITHUB_TOKEN - github token


IMAGE_TAG=$1
IMAGE_REPO=$2
PROTOCOL=docker://

while ! skopeo list-tags "${PROTOCOL}${IMAGE_REPO}" | jq -r '.Tags[]' | grep -q "^${IMAGE_TAG}$"; do
  echo "Waiting for binary image: ${IMAGE_REPO}:${IMAGE_TAG}"
  sleep 5
done
echo "Binary image: ${IMAGE_REPO}:${IMAGE_TAG} available"