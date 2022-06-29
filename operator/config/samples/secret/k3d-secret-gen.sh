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
  config: $(cat /Users/d063994/SAPDevelop/go/kyma-operator/operator/kubeconfigs/kubeconfig--jellyfish--ab-test1.yaml | sed 's/---//g' | base64)
EOF