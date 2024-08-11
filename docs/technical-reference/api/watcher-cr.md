# Watcher Custom Resource

The [Watcher custom resource (CR)](../../../api/v1beta2/watcher_types.go) is vital in setting up the monitoring of changes in specific resources (typically Kyma CR) on the SKR. It is used to configure the virtual service on the control plane for receiving events, and the webhook on the SKR.

## Configuration

### **.spec.serviceInfo**
The serviceInfo field is used to describe the service in the control plane that the Watcher will notify upon detecting changes in the monitored resources. It contains the following fields:

- serviceInfo.name - The name of the service that should be notified.
- serviceInfo.port - The port on which the service is running.
- serviceInfo.namespace - The namespace where the service is deployed.

Example:

```yaml
serviceInfo:
  name: klm-controller-manager-events
  port: 8082
  namespace: kcp-system
```

### **.spec.labelsToWatch**
The labelsToWatch field defines the labels that the Watcher webhook should monitor. The Watcher webhook will observe and report changes from the resources on the SKR that match the these labels.

Example:

```yaml
labelsToWatch:
  "operator.kyma-project.io/watched-by": "lifecycle-manager" 
```

### **.spec.resourceToWatch**
In conjunction with the labelsToWatch field, the resourceToWatch field must also identify the specific resource that the Watcher webhook is supposed monitor. This is defined using the GroupVersionResource (GVR) format.

Example:

```yaml
resourceToWatch:
  group: operator.kyma-project.io
  version: "*"
  resource: kymas
```

**NOTE:** The Watcher webhook monitors the "CREATE", "UPDATE", and "DELETE" operations on these resources.

### **.spec.field**
The field attribute specifies which part of the resource should be monitored. It can be either "spec" to monitor the resource's specification or "status" to monitor the resource's status.

Example:

```yaml
field: "spec"
```

### **.spec.gateway**
The gateway field configures the Istio Gateway that should be used when the Watcher CR triggers the creation or update of a VirtualService. The Gateway is selected using a label selector.

Example:

```yaml
gateway:
  selector:
    matchLabels:
      "operator.kyma-project.io/watcher-gateway": "default"
```

## `operator.kyma-project.io` Labels
- **`operator.kyma-project.io/managed-by`:** This label specifies the module that manages and listens the Watcher CR's corresponding webhook. The value of this label is used to generate the path for the Watcher webhook's requests - `/validate/<label-value>`.