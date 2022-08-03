# template-operator

This documentation serves as a reference to implement a module (component) operator, for integration with the [kyma-operator](https://github.com/kyma-project/kyma-operator/tree/main/operator).
It utilizes the [kubebuilder v3](https://book.kubebuilder.io/) framework to implement Kubernetes APIs for custom resource definitions (CRDs). 
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

### Steps to create a kubebuilder project:

1. Initialize the project. Please make sure the domain is set as `kyma-project.io`.
```
kubebuilder init --domain kyma-project.io --repo github.com/kyma-project/kyma-operator/samples/template-operator
```

2. Create API group version and kind for the intended custom resource(s). Please make sure the `group` is set as `component`.
```
kubebuilder create api --group component --version v1alpha1 --kind Mockup
```

3. `kubebuilder` will ask to create Resource, input `y`.
4. `kubebuilder` will ask to create Controller, input `y`.
5. Update go dependencies `go mod tidy `
6. Run `make generate` followed by `make manifests`, to generate boilerplate code and CRDs respectively.

A basic kubebuilder project with appropriate scaffolding should be setup.

### Status sub-resource for custom resource(s)

A custom resource is required to contain a specific set of properties in the Status object, to be tracked by the [kyma-operator](https://github.com/kyma-project/kyma-operator/tree/main/operator).
This is required to track the current state of the module, represented by this custom resource.

1. Check reference implementation of the [MockupStatus](./api/v1alpha1/mockup_types.go) struct of Mockup custom resource. Similarly `.status.state` property of your custom resource *MUST* contain these state values.
On top, `.status` object could contain other relevant properties as per your requirements.
2. Next, check method `Reconcile()` inside the [Mockup Controller](./controllers/mockup_controller.go), which demonstrates how `.status.state` properties could be set, depending upon your logic.
3. The `.status.state` value has a literal sense behind it, so use them appropriately.