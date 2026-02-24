#!/usr/bin/env bash

# This script creates a PR for sec-scanners-config.yaml version bump
# Note that it writes to stderr for informational messages and only outputs the PR number to stdout,
# which is important for CI usage.
#
# Usage: ./create_sec_scanners_config_bump_pr.sh <version>
#   version: The version being bumped (e.g., "1.3.9")

set -e

if [ -z "$1" ]; then
  echo "Error: Version argument is required" >&2
  echo "Usage: $0 <version>" >&2
  exit 1
fi

VERSION="$1"

# Redirect informational messages to stderr so only PR number goes to stdout
echo "Creating PR for sec-scanners-config.yaml version bump to ${VERSION}" >&2

# Create branch and push changes (redirect output to stderr)
BRANCH="chore/bump-sec-scanners-${VERSION}"
git checkout -b "${BRANCH}" >&2
git add sec-scanners-config.yaml
git commit -m "chore: Bump sec-scanners-config.yaml .bdba image versions to ${VERSION}" >&2
git push origin "${BRANCH}" >&2

# Create PR and extract PR number from the URL returned by gh
PR_URL=$(gh pr create \
  --title "chore: Bump sec-scanners-config.yaml bdba images to ${VERSION}" \
  --body "This PR bumps the sec-scanners-config.yaml bdba image versions to ${VERSION}." \
  --base main \
  --head "${BRANCH}")

# Extract PR number from URL (e.g., https://github.com/owner/repo/pull/123 -> 123)
PR_NUMBER=$(echo "${PR_URL}" | grep -oE '[0-9]+$')

# Output only the PR number to stdout
echo "${PR_NUMBER}"
