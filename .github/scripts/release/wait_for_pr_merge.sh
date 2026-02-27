#!/usr/bin/env bash

# This script waits for a GitHub PR to be merged
#
# Usage: ./wait_for_pr_merge.sh <pr-number>
#   pr-number: The PR number to wait for

set -e

if [ -z "$1" ]; then
  echo "Error: PR number argument is required"
  echo "Usage: $0 <pr-number>"
  exit 1
fi

PR_NUMBER="$1"

echo "Waiting for PR #${PR_NUMBER} to be merged..."

for i in {1..360}; do
  echo "Check ${i}/360: Checking PR status..."
  
  PR_STATE=$(gh pr view "${PR_NUMBER}" \
    --json state -q '.state')
  
  echo "PR #${PR_NUMBER} state: ${PR_STATE}"
  
  if [ "${PR_STATE}" = "MERGED" ]; then
    echo "✅ PR #${PR_NUMBER} is merged! Proceeding with release."
    exit 0
  elif [ "${PR_STATE}" = "CLOSED" ]; then
    echo "❌ PR #${PR_NUMBER} was closed without merging."
    exit 1
  fi
  
  if [ ${i} -lt 60 ]; then
    echo "⏳ PR not merged yet. Waiting 60 seconds before next check..."
    sleep 60
  fi
done

echo "❌ Timeout: PR #${PR_NUMBER} was not merged within 60 minutes."
echo "Please merge the PR and re-run the workflow."
exit 1
