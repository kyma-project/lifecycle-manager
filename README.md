# kyma-operator

Kyma is the opinionated set of Kubernetes based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. Kyma operator is a tool that manages lifecycle of these components in your cluster.

# Architecture

Architecture of kyma-operator and component operators is derived from Kubernetes controllers/operators. Kyma-operator is a meta operator that coordinates lifecycle of kyma components by delegating it to component operators. 

Before you go further please make sure you understand concepts of Kubernetes API and resources. Recommended reading:
- [Kubebuilder book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/docs/building-operators/golang/)

The architecture is based on best practices for building Kubernetes operators ([1](https://cloud.google.com/blog/products/containers-kubernetes/best-practices-for-building-kubernetes-operators-and-stateful-apps), [2](https://sdk.operatorframework.io/docs/best-practices/)). 

Some architecture decisions were derived from business requirements and experiments (proof of concepts):
[Architecture Decisions](docs/architecture-decisions.md)

![](docs/assets/kyma-operator-architecture.svg)

Kyma-operator manages Kyma custom resource (CR). Kyma custom resource contains version of Kyma and list of components. Kyma-operator creates and watches component custom resources and updates Kyma resource status. Component operators watch only own custom resources, reconcile components in the target clusters to the desired states and update component resource status.

## Example

Create resource Kyma:
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

Such resource triggers kyma-operator, that creates 2 custom resources: `ServerlessComponent` and `IstioConfiguration`. These custom resources will trigger serverless-operator and istio-operator. When each component operator completes the installation it updates own resource status (`ServerlessComponent` and `IstioConfiguration`). Status changes trigger kyma-operator to update `Kyma` resource status. 