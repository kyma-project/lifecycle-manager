#! /bin/bash

enableSync="true"
if [ "$1" = "singlecluster" ]; then
  enableSync="false"
fi

touch kyma.yaml
cat <<EOF > kyma.yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  name: kyma-sample
  namespace: $(yq '.metadata.namespace' template.yaml)
spec:
  channel: $(yq '.spec.channel' template.yaml)
  sync:
    enabled: $enableSync
  modules:
    - name: $(yq '.metadata.labels | with_entries(select(.key == "operator.kyma-project.io/module-name")) | .[]' template.yaml)
EOF



