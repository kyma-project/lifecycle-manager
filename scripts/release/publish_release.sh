#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

RELEASE_ID=$1

GITHUB_URL=https://api.github.com/repos/$CODE_REPOSITORY
GITHUB_AUTH_HEADER="Authorization: Bearer $GITHUB_TOKEN"

CURL_RESPONSE=$(curl -L \
  -s \
  --fail-with-body \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "$GITHUB_AUTH_HEADER" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "$GITHUB_URL"/releases/"$RELEASE_ID" \
  -d '{"draft":false}')

echo "$CURL_RESPONSE"

