# kyma-operator

Kyma is the opinionated set of Kubernetes based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma operator is a tool that manages the lifecycle of these components in your cluster.

# Architecture

The architecture of `kyma-operator` and component operators is based on Kubernetes controllers/operators. `kyma-operator` is a meta operator that coordinates and tracks the lifecycle of kyma components by delegating it to component operators.

Before you go further please make sure you understand concepts of Kubernetes API and resources. Recommended reading:
- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)

The architecture is based on best practices for building Kubernetes operators ([1](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps), [2](https://sdk.operatorframework.io/docs/best-practices/)). 

Some architecture decisions were derived from business requirements and experiments (proof of concepts):
[Architecture Decisions](docs/architecture-decisions.md)

![](docs/assets/kyma-operator-architecture.svg)

`kyma-operator ` manages Clusters through the `Kyma` custom resource (CR). `Kyma` contains a desired state of all components in a cluster for a given Kyma Release. 

`kyma-operator` creates component custom resources and updates `Kyma`'s status subresource based on the observed status changes in the component custom resource (similar to a deployment tracking pods). 

Component operators watch only their own custom resources and reconcile components in the target clusters to the desired state. These states are then aggregated in `Kyma` to reflect the cluster state.

## Example

A sample `Kyma` CR could look like this:
```
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  name: kyma-sample
spec:
  version: 2.2
  kubeconfigName: cluster1-kubeconfig
  components:
  - name: istio
  - name: serverless
```

The creation of the custom resource triggers a reconciliation of kyma-operator, that creates 2 custom resources: `ServerlessComponent` and `IstioConfiguration` based on a template. These custom resources will trigger serverless-operator and istio-operator. When each component operator completes the installation it updates it's own resource status (`ServerlessComponent/status` and `IstioConfiguration/status`). Status changes trigger kyma-operator to update `Kyma` resource status and aggregate and combine the readiness condition of the cluster.