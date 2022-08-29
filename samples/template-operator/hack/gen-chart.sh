#! /bin/bash
echo "
apiVersion: v2
name: $MODULE_NAME-operator
description: A Helm chart for the Operator in a Cluster based on a Kustomize Manifest
type: application
version: $MODULE_VERSION
appVersion: "$MODULE_VERSION"
"