#!/bin/bash

GARDENER_CERT_MANAGER_VERSION=$1
GARDENER_CERT_MANAGER_RENEWAL_WINDOW=$2

helm install cert-controller-manager \
   oci://europe-docker.pkg.dev/gardener-project/releases/charts/cert-controller-manager \
   --version v"$GARDENER_CERT_MANAGER_VERSION" \
   --set configuration.renewalWindow="$GARDENER_CERT_MANAGER_RENEWAL_WINDOW"
