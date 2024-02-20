#!/usr/bin/env bash

set -o nounset
set -o pipefail

RELEASE_TAG=$1

GITHUB_URL=https://api.github.com/repos/$CODE_REPOSITORY
GITHUB_AUTH_HEADER="Authorization: Bearer $GITHUB_TOKEN"

curl -L \
    -s \
    --fail-with-body \
    -H "Accept: application/vnd.github+json" \
    -H "$GITHUB_AUTH_HEADER" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$GITHUB_URL"/releases/tags/"$RELEASE_TAG"

CURL_EXIT_CODE=$?

if [[ $CURL_EXIT_CODE == 0 ]]; then
    echo "Release with tag: $RELEASE_TAG already exists!"
    exit 1
fi

exit 0
