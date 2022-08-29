# Architecture decisions

## Deployment models

lifecycle-manager (and component operators) can run in following modes:

- in-cluster - regular deployment in the kubernetes cluster where kyma should be deployed
- control-plane - deployment on central kubernetes cluster that manages multiple kyma installations (installing kyma on the remote clusters)
- cli - local binary using kubeconfig to install kyma components on target cluster

## Separate repositories for component operators

Teams providing component operators should work (and release) independently from main lifecycle-manager and other component operators.

## API in separate module

Kubernetes API defined by operator (Custom Resource Definition types) should be placed in separate go module. It can be used to generate client and use API in other modules without importing all dependencies used by the implementation.

## Dynamic client in lifecycle-manager

lifecycle-manager should not have hard-coded dependencies to any component operator. lifecycle-manager should create and watch custom resources for components without importing their code. Resource templates should be stored in the configuration (config maps).

## Kyma versions

Kyma version will be part of `Kyma` custom resource spec. Templates for component resources (config maps) should be labeled with the version and component name:

```
operator.kyma-project.io/component-name=serverless
operator.kyma-project.io/version=2.2
```

To avoid config map pollution regular cleanup should be performed (versions that are no longer supported on production should be removed). Consider also some fallback for patch releases (if the 2.2.7 version is not found use the latest 2.2.x version).
