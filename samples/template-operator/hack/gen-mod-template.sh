#! /bin/bash
touch $MODULE_TEMPLATE
cat <<EOF > $MODULE_TEMPLATE
apiVersion: operator.kyma-project.io/v1alpha1
kind: ModuleTemplate
metadata:
  name: moduletemplate-$MODULE_NAME
  namespace: default
  labels:
    "operator.kyma-project.io/managed-by": "${OPERATOR_NAME}"
    "operator.kyma-project.io/controller-name": "manifest"
    "operator.kyma-project.io/module-name": "$(basename $MODULE_NAME)"
  annotations:
    "operator.kyma-project.io/module-version": "$(yq e ".component.version" $REMOTE_SIGNED_DESCRIPTOR)"
    "operator.kyma-project.io/module-provider": "$(yq e ".component.provider" $REMOTE_SIGNED_DESCRIPTOR)"
    "operator.kyma-project.io/descriptor-schema-version": "$(yq e ".meta.schemaVersion" $REMOTE_SIGNED_DESCRIPTOR)"
    "operator.kyma-project.io/control-signature-name": "$(yq e ".signatures[0].name" $REMOTE_SIGNED_DESCRIPTOR)"
    "operator.kyma-project.io/generated-at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
spec:
  channel: ${MODULE_TEMPLATE_CHANNEL}
  data:
$(cat $DEFAULT_DATA | sed "s/^/    /g")
  descriptor:
$(cat $REMOTE_SIGNED_DESCRIPTOR | sed "s/^/    /g")
EOF
