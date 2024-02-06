#!/bin/bash

# This script deploys the storage version migrator. The migrator deploys its resources in the `kube-system` namespace
# and it gets triggered every 10 minutes to migrate all resources stored in the etcd to the latest storage version.

# To run the script, use the following command:
# `sh ./scripts/storage_version_migrator.sh -c ${CONTEXT}`
# where CONTEXT is the context for the cluster where you want to do the API migration

while getopts c: flag

do
        case "${flag}" in
                c) context=${OPTARG};;
                *) echo "Invalid option: -$flag" ;;
        esac
done

git clone https://github.com/kubernetes-sigs/kube-storage-version-migrator.git
cd kube-storage-version-migrator
kubectl config use-context $context
make local-manifests REGISTRY=eu.gcr.io/k8s-artifacts-prod/storage-migrator VERSION=v0.0.5
pushd manifests.local
kubectl apply -k ./
popd
