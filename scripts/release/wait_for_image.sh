#!/usr/bin/env bash

set -o nounset
set -o errexit
set -E
set -o pipefail

DOCKER_IMAGE=$1
ITERATIONS=${2:-30}
SLEEP_TIME="${3:-30}"

for (( c=1; c<=ITERATIONS; c++ ))
do
    if docker manifest inspect "$DOCKER_IMAGE" > /dev/null 2>&1; then
	    exit 0
    fi
    echo "Attempt $c: Docker image: $DOCKER_IMAGE doesn't exist"
    if [[ $c -lt $ITERATIONS ]]; then
        sleep "$SLEEP_TIME"
    fi
done

echo "Fail: Docker image: $DOCKER_IMAGE doesn't exist"
exit 1
