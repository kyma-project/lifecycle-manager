#! /bin/bash
echo "
apiVersion: v2
name: $MODULE_NAME-operator
description: A Helm chart for the Operator in a Cluster
type: application
version: v$MODULE_VERSION
appVersion: "$MODULE_VERSION"
"