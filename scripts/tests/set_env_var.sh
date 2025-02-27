#!/bin/bash
set -o pipefail
set -o errexit

# if GITHUB_ACTIONS is not set, set $1=$2. $1 is the name of the global env var and $2 is the value
if [ -z "${GITHUB_ACTIONS}" ]; then
  eval "$1=$2"
fi
