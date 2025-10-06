# Kyma Custom Resource

<!-- Kyma Custom Resources Definition for the end user-->

The `kymas.operator.kyma-project.io` Custom Resource Definition (CRD) is a comprehensive specification that defines the structure and format used to manage a cluster and its desired state. It contains the list of added modules and their state.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd kymas.operator.kyma-project.io -o yaml
```

For more information on the fields and how to use them, see [Kyma](../contributor/resources/01-kyma.md).
