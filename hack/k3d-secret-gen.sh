#! /bin/bash
echo "
apiVersion: v1
kind: Secret
metadata:
  name:  $1
  namespace: $2
  labels:
    operator.kyma-project.io/managed-by: lifecycle-manager
    operator.kyma-project.io/kyma-name:  $1
type: Opaque
data:
  config: $(kubectl config view --raw --minify | sed 's/---//g' | base64)" > $1-secret.yaml
