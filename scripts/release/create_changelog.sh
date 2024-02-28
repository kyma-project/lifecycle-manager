#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

CURRENT_RELEASE_TAG=$1
LAST_RELEASE_TAG=$2
DOCKER_IMAGE_URL=$3

GITHUB_URL=https://api.github.com/repos/$CODE_REPOSITORY
GITHUB_AUTH_HEADER="Authorization: Bearer $GITHUB_TOKEN"
CHANGELOG_FILE="CHANGELOG.md"

echo "## What has changed" >> $CHANGELOG_FILE

git log "$LAST_RELEASE_TAG..$CURRENT_RELEASE_TAG" --pretty=tformat:"%h" --reverse | while read -r commit
do
    COMMIT_AUTHOR=$(curl -H "$GITHUB_AUTH_HEADER" -sS "$GITHUB_URL"/commits/"$commit" | jq -r '.author.login')
    git show -s "$commit" --format="* %s by @$COMMIT_AUTHOR" >> $CHANGELOG_FILE
done

{
    echo -e "\n**Full changelog**: $GITHUB_URL/compare/$LAST_RELEASE_TAG...$CURRENT_RELEASE_TAG"
    echo -e "\n"
    echo "## Docker image URL"
    echo "$DOCKER_IMAGE_URL" 
} >> $CHANGELOG_FILE

echo $CHANGELOG_FILE
