indent() {
  local indentSize=1
  local indent=1
  if [ -n "$1" ]; then indent=$1; fi
  pr -to $(($indent * $indentSize))
}

COMPONENT_ARCHIVE="./example"
COMPONENT_RESOURCES="./resources.yaml"
COMPONENT_NAME="kyma-project.io/module/example"
COMPONENT_VERSION="v0.0.1"
REGISTRY_URL="operator-test-registry.localhost:50241"
PRIVATE_KEY="./private-key.pem"
PUBLIC_KEY="./public-key.pem"
SIGNATURE_NAME="test-signature"

REMOTE_DESCRIPTOR="remote-component-descriptor.yaml"
REMOTE_SIGNED_DESCRIPTOR="remote-component-descriptor-signed.yaml"

component-cli component-archive create ${COMPONENT_ARCHIVE} --component-name ${COMPONENT_NAME} --component-version ${COMPONENT_VERSION}
component-cli ca resources add ${COMPONENT_ARCHIVE} ${COMPONENT_RESOURCES}
component-cli ca remote push example --repo-ctx ${REGISTRY_URL}

openssl genpkey -algorithm RSA -out ${PRIVATE_KEY}
openssl rsa -in ${PRIVATE_KEY} -pubout > ${PUBLIC_KEY}
component-cli ca signatures sign rsa ${REGISTRY_URL} ${COMPONENT_NAME} ${COMPONENT_VERSION} --upload-base-url ${REGISTRY_URL}/signed --recursive --signature-name ${SIGNATURE_NAME} --private-key ${PRIVATE_KEY}
component-cli ca signatures verify rsa ${REGISTRY_URL}/signed ${COMPONENT_NAME} ${COMPONENT_VERSION} --signature-name ${SIGNATURE_NAME} --public-key ${PUBLIC_KEY}

component-cli ca remote get ${REGISTRY_URL} ${COMPONENT_NAME} ${COMPONENT_VERSION} >> ${REMOTE_DESCRIPTOR}
component-cli ca remote get ${REGISTRY_URL}/signed ${COMPONENT_NAME} ${COMPONENT_VERSION} >> ${REMOTE_SIGNED_DESCRIPTOR}

cat <<EOF > ./generated-module-template.yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample-manifest
  namespace: default
  labels:
    "operator.kyma-project.io/managed-by": "kyma-operator"
    "operator.kyma-project.io/controller-name": "manifest"
  annotations:
    "operator.kyma-project.io/module-name": "$(yq e ".component.name" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/module-version": "$(yq e ".component.version" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/module-provider": "$(yq e ".component.provider" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/descriptor-schema-version": "$(yq e ".meta.schemaVersion" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/control-signature-name": "$(yq e ".signatures[0].name" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/control-signature-algorithm": "$(yq e ".signatures[0].digest.hashAlgorithm" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/control-signature-value": "$(yq e ".signatures[0].digest.value" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/generated-at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
spec:
  channel: stable
  descriptor:
$(cat ${REMOTE_SIGNED_DESCRIPTOR} | indent 4)
EOF

