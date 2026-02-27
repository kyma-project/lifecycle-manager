#!/usr/bin/env bash

# This script deletes the branch created for sec-scanners-config.yaml version bump
#
# Usage: ./delete_sec_scanners_config_pr_branch.sh <version>
#   version: The version that was bumped (e.g., "1.3.9")

set -e

if [ -z "$1" ]; then
  echo "Error: Version argument is required"
  echo "Usage: $0 <version>"
  exit 1
fi

VERSION="$1"
BRANCH="chore/bump-sec-scanners-${VERSION}"

echo "Deleting merged branch: ${BRANCH}"
git push origin --delete "${BRANCH}" || echo "Branch ${BRANCH} already deleted or does not exist"
echo "✅ Merged PR branch deleted: ${BRANCH}"
