#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

RELEASE_TAG=$1
CHANGELOG_FILE_NAME=$2
CHANGELOG_FILE=$(cat "$CHANGELOG_FILE_NAME")

GITHUB_URL=https://api.github.com/repos/$CODE_REPOSITORY
GITHUB_AUTH_HEADER="Authorization: Bearer $GITHUB_TOKEN"

JSON_PAYLOAD=$(jq -n \
  --arg tag_name "$RELEASE_TAG" \
  --arg name "$RELEASE_TAG" \
  --arg body "$CHANGELOG_FILE" \
  '{
    "tag_name": $tag_name,
    "name": $name,
    "body": $body,
    "draft": true
  }')

CURL_RESPONSE=$(curl -L \
  -s \
  --fail-with-body \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "$GITHUB_AUTH_HEADER" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "$GITHUB_URL"/releases \
  -d "$JSON_PAYLOAD")

# return the id of the release draft
echo "$CURL_RESPONSE" | jq -r ".id"

