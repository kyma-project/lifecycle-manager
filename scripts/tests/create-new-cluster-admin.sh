#!/bin/bash
# create-new-cluster-admin.sh <username>
# Example: ./create-new-cluster-admin.sh alice

USER_NAME=$1
KUBECONFIG_OUT="$1-kubeconfig.yaml"

# Extract cluster info from current kubeconfig
CURRENT_CONTEXT=$(kubectl config current-context)
CLUSTER_NAME=$(kubectl config view -o jsonpath="{.contexts[?(@.name==\"$CURRENT_CONTEXT\")].context.cluster}")
CLUSTER_SERVER=$(kubectl config view -o jsonpath="{.clusters[?(@.name==\"$CLUSTER_NAME\")].cluster.server}")
CA_DATA=$(kubectl config view --raw -o jsonpath="{.clusters[?(@.name==\"$CLUSTER_NAME\")].cluster.certificate-authority-data}")

# Generate key + CSR for the new user
openssl genrsa -out ${USER_NAME}.key 2048
openssl req -new -key ${USER_NAME}.key -out ${USER_NAME}.csr -subj "/CN=${USER_NAME}/O=dev"

# Create Kubernetes CSR object
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${USER_NAME}
spec:
  request: $(cat ${USER_NAME}.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

# Approve CSR and fetch the signed cert
kubectl certificate approve ${USER_NAME}
kubectl get csr ${USER_NAME} -o jsonpath='{.status.certificate}' \
  | base64 --decode > ${USER_NAME}.crt

# Build kubeconfig for the new user
TMP_CA="$(mktemp)"
echo "${CA_DATA}" | base64 --decode > "$TMP_CA"

kubectl config set-cluster ${CLUSTER_NAME} \
  --server=${CLUSTER_SERVER} \
  --certificate-authority="$TMP_CA" \
  --embed-certs=true \
  --kubeconfig=${KUBECONFIG_OUT}

kubectl config set-credentials ${USER_NAME}@${CLUSTER_NAME} \
  --client-certificate=${USER_NAME}.crt \
  --client-key=${USER_NAME}.key \
  --embed-certs=true \
  --kubeconfig=${KUBECONFIG_OUT}

kubectl config set-context ${USER_NAME}@${CLUSTER_NAME} \
  --cluster=${CLUSTER_NAME} \
  --user=${USER_NAME}@${CLUSTER_NAME} \
  --kubeconfig=${KUBECONFIG_OUT}

kubectl config use-context ${USER_NAME}@${CLUSTER_NAME} --kubeconfig=${KUBECONFIG_OUT}

# 🔑 Grant cluster-admin rights
kubectl create clusterrolebinding ${USER_NAME}-cluster-admin \
  --clusterrole=cluster-admin \
  --user=${USER_NAME} \
  --dry-run=client -o yaml | kubectl apply -f -

# compute absolute path to kubeconfig output (portable)
KUBECONFIG_OUT_ABS="$(cd "$(dirname "$KUBECONFIG_OUT")" && pwd)/$(basename "$KUBECONFIG_OUT")"


echo "✅ User ${USER_NAME} created and granted cluster-admin rights."
echo "👉 Kubeconfig written to ${KUBECONFIG_OUT_ABS}"