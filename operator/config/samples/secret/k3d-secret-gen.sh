cat <<EOF > ./../component-integration-installed/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample #change with your kyma name
type: Opaque
data:
  config: $(k3d kubeconfig get kyma | sed 's/---//g' | base64)
EOF