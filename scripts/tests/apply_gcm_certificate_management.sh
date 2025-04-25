#!/bin/bash

GARDENER_CERT_MANAGER_VERSION=$1
GARDENER_CERT_MANAGER_RENEWAL_WINDOW=$2

helm install cert-controller-manager \
   oci://europe-docker.pkg.dev/gardener-project/releases/charts/cert-controller-manager \
   --version v"$GARDENER_CERT_MANAGER_VERSION" \
   --set configuration.renewalWindow="$GARDENER_CERT_MANAGER_RENEWAL_WINDOW"

# this is needed for GCM to work since this is not included in the helm chart for GCM
kubectl apply -f https://raw.githubusercontent.com/gardener/cert-management/master/examples/11-dns.gardener.cloud_dnsentries.yaml
