indent() {
  local indentSize=1
  local indent=1
  if [ -n "$1" ]; then indent=$1; fi
  pr -to $(($indent * $indentSize))
}

HOSTS_FILE="/etc/hosts"
COMPONENT_ARCHIVE="./example"
COMPONENT_RESOURCES="./resources.yaml"
PRIVATE_KEY="./private-key.pem"
PUBLIC_KEY="./public-key.pem"
REMOTE_DESCRIPTOR="./remote-component-descriptor.yaml"
REMOTE_SIGNED_DESCRIPTOR="./remote-component-descriptor-signed.yaml"
MODULE_TEMPLATE="./generated-module-template.yaml"

COMPONENT_NAME="kyma-project.io/module/example"
COMPONENT_VERSION="v0.0.10"
REGISTRY_NAME="operator-test-registry"
REGISTRY_HOST="${REGISTRY_NAME}.localhost"
REGISTRY_URL="${REGISTRY_HOST}:50241"
SIGNATURE_NAME="test-signature"

if k3d registry get ${REGISTRY_NAME} | grep -q ${REGISTRY_NAME}; then
   echo "OCI Registry ${REGISTRY_NAME} Exists, continuing..."
else
   echo "OCI Registry ${REGISTRY_NAME} does not exist!"
   exit 1
fi

if grep -q "${REGISTRY_HOST}" ${HOSTS_FILE}; then
    echo "${REGISTRY_HOST} is listed in host file, continuing..."
else
    echo "${REGISTRY_HOST} Doesn't exist in ${HOSTS_FILE}, will attempt to add it"
    echo "127.0.0.1 ${REGISTRY_HOST}" >> ${HOSTS_FILE}
fi

rm -r ${COMPONENT_ARCHIVE}

component-cli component-archive create ${COMPONENT_ARCHIVE} --component-name ${COMPONENT_NAME} --component-version ${COMPONENT_VERSION}
component-cli ca resources add ${COMPONENT_ARCHIVE} ${COMPONENT_RESOURCES}
component-cli ca remote push ${COMPONENT_ARCHIVE} --repo-ctx ${REGISTRY_URL}

rm ${PRIVATE_KEY}
openssl genpkey -algorithm RSA -out ${PRIVATE_KEY}
rm ${PUBLIC_KEY}
openssl rsa -in ${PRIVATE_KEY} -pubout > ${PUBLIC_KEY}
component-cli ca signatures sign rsa ${REGISTRY_URL} ${COMPONENT_NAME} ${COMPONENT_VERSION} --upload-base-url ${REGISTRY_URL}/signed --recursive --signature-name ${SIGNATURE_NAME} --private-key ${PRIVATE_KEY}
component-cli ca signatures verify rsa ${REGISTRY_URL}/signed ${COMPONENT_NAME} ${COMPONENT_VERSION} --signature-name ${SIGNATURE_NAME} --public-key ${PUBLIC_KEY}

rm ${REMOTE_DESCRIPTOR}
component-cli ca remote get ${REGISTRY_URL} ${COMPONENT_NAME} ${COMPONENT_VERSION} >> ${REMOTE_DESCRIPTOR}
rm ${REMOTE_SIGNED_DESCRIPTOR}
component-cli ca remote get ${REGISTRY_URL}/signed ${COMPONENT_NAME} ${COMPONENT_VERSION} >> ${REMOTE_SIGNED_DESCRIPTOR}

echo "Successfully generated Remote Descriptors in ${REMOTE_DESCRIPTOR} (Signed Version at ${REMOTE_SIGNED_DESCRIPTOR})"

cat <<EOF > ${MODULE_TEMPLATE}
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

echo "Generated ModuleTemplate at ${MODULE_TEMPLATE}"