#!/usr/bin/env bash


set -o errexit
set -E
set -o pipefail

CURRENT_RELEASE_TAG=$1
DOCKER_IMAGE_URL=$2
LAST_RELEASE_TAG=$3

if [ "${LAST_RELEASE_TAG}"  == "" ]
then
  LAST_RELEASE_TAG=$(git describe --tags --abbrev=0)
fi

GITHUB_URL=https://api.github.com/repos/$CODE_REPOSITORY
GITHUB_AUTH_HEADER="Authorization: Bearer $GITHUB_TOKEN"
CHANGELOG_FILE="CHANGELOG.md"

git log "$LAST_RELEASE_TAG"..HEAD --pretty=tformat:"%h" --reverse | while read -r commit
do
    COMMIT_AUTHOR=$(curl -H "$GITHUB_AUTH_HEADER" -sS "$GITHUB_URL"/commits/"$commit" | jq -r '.author.login')
    if [ "${COMMIT_AUTHOR}" != "kyma-bot" ]; then
      git show -s "${commit}" --format="* %s by @${COMMIT_AUTHOR}" >> ${CHANGELOG_FILE}
    fi
done

{
    echo -e "\n**Full changelog**: $GITHUB_URL/compare/$LAST_RELEASE_TAG...$CURRENT_RELEASE_TAG"
    echo -e "\n"
    echo "## Docker image URL"
    echo "$DOCKER_IMAGE_URL"
} >> $CHANGELOG_FILE

cat $CHANGELOG_FILE
