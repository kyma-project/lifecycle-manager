#!/bin/sh
while getopts c: flag

do
        case "${flag}" in
                c) context=${OPTARG};;
                *) echo "Invalid option: -$flag" ;;
        esac
done

if [[ -n "$context" ]]; then
  kubectl config use-context $context
fi

kubectl delete clusterrolebinding storage-version-migration-migrator
kubectl delete clusterrolebinding storage-version-migration-trigger
kubectl delete clusterrolebinding storage-version-migration-crd-creator
kubectl delete clusterrolebinding storage-version-migration-initializer

kubectl delete clusterrole storage-version-migration-crd-creator
kubectl delete clusterrole storage-version-migration-initializer
kubectl delete clusterrole storage-version-migration-trigger

kubectl delete deployment migrator -n kube-system
kubectl delete deployment trigger -n kube-system

kubectl delete crd storageversionmigrations.migration.k8s.io
kubectl delete crd storagestates.migration.k8s.io
