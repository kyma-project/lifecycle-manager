# Watcher

The `watchers.operator.kyma-project.io` Custom Resource Definition (CRD) defines the structure and format used to configure the Watcher resource.

The Watcher custom resource (CR) defines the callback functionality for synchronized Kyma runtime clusters, that allows lower latencies before the Kyma Control Plane cluster detects any changes.

The full Watcher CR documentation van be found [here.](https://github.com/kyma-project/runtime-watcher/blob/main/docs/api.md)

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd watchers.operator.kyma-project.io -o yaml
```
