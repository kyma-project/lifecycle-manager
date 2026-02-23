#!/usr/bin/env bash

# This script creates a PR for sec-scanners-config.yaml version bump
#
# Usage: ./create_sec_scanners_config_bump_pr.sh <version>
#   version: The version being bumped (e.g., "1.3.9")

set -e

if [ -z "$1" ]; then
  echo "Error: Version argument is required"
  echo "Usage: $0 <version>"
  exit 1
fi

VERSION="$1"

echo "Creating PR for sec-scanners-config.yaml version bump to ${VERSION}"

# Configure git
git config --local user.email "jellyfish-bot@users.noreply.github.com"
git config --local user.name "jellyfish-bot"

# Create branch and push changes
BRANCH="chore/bump-sec-scanners-${VERSION}"
git checkout -b "${BRANCH}"
git add sec-scanners-config.yaml
git commit -m "chore: Bump sec-scanners-config.yaml .bdba image versions to ${VERSION}" || echo "No changes to commit"
git push origin "${BRANCH}"

# Create PR
PR_URL=$(gh pr create \
  --title "chore: Bump sec-scanners-config.yaml .bdba image versions to ${VERSION}" \
  --body "Automated version bump of \`sec-scanners-config.yaml\` to \`${VERSION}\`." \
  --base main \
  --head "${BRANCH}")

# Extract PR number and return it to caller
PR_NUMBER=$(echo "${PR_URL}" | grep -oE '[0-9]+$')
echo "✅ PR created: ${PR_URL}"
echo "${PR_NUMBER}"
