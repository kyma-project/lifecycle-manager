#!/bin/bash

# Exporting the path to the kubeconfig file
export KUBECONFIG=$HOME/.k3d/kcp-local.yaml

for i in {1..100}; do
  cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-cert-$i
  namespace: istio-system
spec:
  commonName: test-cert-$i
  subject:
    organizationalUnits:
      - "BTP Kyma Runtime"
    organizations:
      - "SAP SE"
    localities:
      - "Walldorf"
    provinces:
      - "Baden-Württemberg"
    countries:
      - "DE"
  secretName: test-cert-$i
  privateKey:
    rotationPolicy: Always
    algorithm: RSA
    size: 4096
  issuerRef:
    name: klm-watcher-root
    kind: Issuer
    group: cert-manager.io
  duration: 2160h # 90d
  renewBefore: 1464h # 61d
EOF

  if [ $? -ne 0 ]; then
    echo "[$(basename "$0")] Cert deployment failed for test-cert-$i"
    exit 1
  fi

done

echo "[$(basename "$0")] All certs deployed successfully"
