indent() {
  local indentSize=1
  local indent=1
  if [ -n "$1" ]; then indent=$1; fi
  pr -to $(($indent * $indentSize))
}

CLI=$(which component-cli)

HOSTS_FILE="/etc/hosts"

OPERATOR_NAME="kyma-operator"

COMPONENT_ARCHIVE="./example"
DATA_DIR="./data"
COMPONENT_RESOURCES="./resources.yaml"

PRIVATE_KEY="./private-key.pem"
PUBLIC_KEY="./public-key.pem"
PUBLIC_KEY_VERIFICATION_SECRET="./public-key-secret.yaml"
SIGNATURE_NAME="kyma-module-signature"

REMOTE_DESCRIPTOR="./remote-component-descriptor.yaml"
REMOTE_SIGNED_DESCRIPTOR="./remote-component-descriptor-signed.yaml"

MODULE_TEMPLATE="./generated-module-template.yaml"
MODULE_TEMPLATE_CHANNEL="stable"
MODULE_NAME="kyma-project.io/module/skr-module"
MODULE_VERSION="v0.0.65"
MODULE_PROFILE="production"

# this requires a k3d registry with a cluster
# e.g. k3d cluster create operator-test --registry-create operator-test-registry.localhost:50241
REGISTRY_NAME="operator-test-registry"
REGISTRY_HOST="${REGISTRY_NAME}.localhost"
REGISTRY_URL="${REGISTRY_HOST}:50241"

helm repo add nginx-stable https://helm.nginx.com/stable
helm pull nginx-stable/nginx-ingress --untar --untardir ${DATA_DIR}

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

rm -rf ${COMPONENT_ARCHIVE}

$CLI component-archive create ${COMPONENT_ARCHIVE} --component-name ${MODULE_NAME} --component-version ${MODULE_VERSION}
$CLI ca resources add ${COMPONENT_ARCHIVE} ${COMPONENT_RESOURCES}
$CLI ca remote push ${COMPONENT_ARCHIVE} --repo-ctx ${REGISTRY_URL}

rm -rf ${PRIVATE_KEY}
openssl genpkey -algorithm RSA -out ${PRIVATE_KEY}
rm -rf ${PUBLIC_KEY}
openssl rsa -in ${PRIVATE_KEY} -pubout > ${PUBLIC_KEY}
$CLI ca signatures sign rsa ${REGISTRY_URL} ${MODULE_NAME} ${MODULE_VERSION} --upload-base-url ${REGISTRY_URL}/signed --recursive --signature-name ${SIGNATURE_NAME} --private-key ${PRIVATE_KEY}
$CLI ca signatures verify rsa ${REGISTRY_URL}/signed ${MODULE_NAME} ${MODULE_VERSION} --signature-name ${SIGNATURE_NAME} --public-key ${PUBLIC_KEY}

cat <<EOF > ${PUBLIC_KEY_VERIFICATION_SECRET}
apiVersion: v1
kind: Secret
metadata:
  name: kyma-signature-check #change with your kyma name
  labels:
    "operator.kyma-project.io/managed-by": "${OPERATOR_NAME}"
    "operator.kyma-project.io/signature": "${SIGNATURE_NAME}"
type: Opaque
data:
  key: $(cat ${PUBLIC_KEY} | base64 -w 0)
EOF

echo "Public Key successfully generated as secret:"
kubectl apply -f ${PUBLIC_KEY_VERIFICATION_SECRET}

rm -rf ${REMOTE_DESCRIPTOR}
$CLI ca remote get ${REGISTRY_URL} ${MODULE_NAME} ${MODULE_VERSION} >> ${REMOTE_DESCRIPTOR}
rm -rf ${REMOTE_SIGNED_DESCRIPTOR}
$CLI ca remote get ${REGISTRY_URL}/signed ${MODULE_NAME} ${MODULE_VERSION} >> ${REMOTE_SIGNED_DESCRIPTOR}

echo "Successfully generated Remote Descriptors in ${REMOTE_DESCRIPTOR} (Signed Version at ${REMOTE_SIGNED_DESCRIPTOR})"

cat <<EOF > ${MODULE_TEMPLATE}
apiVersion: operator.kyma-project.io/v1alpha1
kind: ModuleTemplate
metadata:
  name: moduletemplate-sample-manifest
  namespace: default
  labels:
    "operator.kyma-project.io/managed-by": "${OPERATOR_NAME}"
    "operator.kyma-project.io/controller-name": "manifest"
    "operator.kyma-project.io/module-name": "$(basename $MODULE_NAME)"
    "operator.kyma-project.io/profile": "${MODULE_PROFILE}"
  annotations:
    "operator.kyma-project.io/module-version": "$(yq e ".component.version" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/module-provider": "$(yq e ".component.provider" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/descriptor-schema-version": "$(yq e ".meta.schemaVersion" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/control-signature-name": "$(yq e ".signatures[0].name" remote-component-descriptor-signed.yaml)"
    "operator.kyma-project.io/generated-at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
spec:
  remote: true
  channel: ${MODULE_TEMPLATE_CHANNEL}
  data:
    kind: Manifest
    resource: manifests
    apiVersion: component.kyma-project.io/v1alpha1
  descriptor:
$(cat ${REMOTE_SIGNED_DESCRIPTOR} | indent 4)
EOF

echo "Generated ModuleTemplate at ${MODULE_TEMPLATE}"
kubectl apply -f ${MODULE_TEMPLATE}