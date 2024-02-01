#!/bin/bash
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

