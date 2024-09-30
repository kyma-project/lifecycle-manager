# Watcher

The `watchers.operator.kyma-project.io` Custom Resource Definition (CRD) is a comprehensive specification that defines the structure and format used to configure the Watcher resource.

The Watcher custom resource (CR) defines the callback functionality for synchronized remote clusters that allows lower latencies before the Control Plane detects any changes.

To get the latest CRD in the YAML format, run the following command:

```bash
kubectl get crd watchers.operator.kyma-project.io -o yaml
```
