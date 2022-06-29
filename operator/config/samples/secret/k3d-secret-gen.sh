cat <<EOF > ./../component-integration-installed/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample #change with your kyma name
  labels:
    "operator.kyma-project.io/managed-by": "kyma-operator"
    "operator.kyma-project.io/kyma-name": "kyma-sample"
type: Opaque
data:
  config: $(cat k3d kubeconfig get kyma | sed 's/---//g' | base64)
EOF