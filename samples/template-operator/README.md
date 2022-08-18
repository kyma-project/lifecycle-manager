# template-operator

This documentation serves as a reference to implement a module (component) operator, for integration with the [kyma-operator](https://github.com/kyma-project/kyma-operator/tree/main/operator).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for custom resource definitions (CRDs). 
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

## Structure

The template operator contains base scaffolding that is prepared to build a Kyma Module from the various commands in `Makefile`.

```
Usage:
  make <target>

General
  help                   Display this help.

Module
  module-operator-chart  Bundle the Module Operator Chart
  module-image           Build the Module Image and push it to a registry defined in IMG_REGISTRY
  module-build           Build the Module and push it to a registry defined in MODULE_REGISTRY
  module-default         Bootstrap the Default CR

Tools
  kyma                   Download & Build Kyma CLI locally if necessary.
  kustomize              Download & Build kustomize locally if necessary.
  component-cli          Download & Build kustomize locally if necessary.
  grafana-dashboard      Generating Grafana manifests to visualize controller status.
```

To use the Makefile you will need to adjust your Module information to make sure that the Makefile knows the correct remotes / targets.

```makefile
# Module Name used for bundling the OCI Image and later on for referencing in the Kyma Modules
MODULE_NAME ?= template
# Semantic Module Version used for identifying the build
MODULE_VERSION ?= 0.0.0
# Module Registry used for pushing the image
MODULE_REGISTRY ?= op-kcp-registry:56888/unsigned
# Desired Channel of the Generated Module Template
MODULE_TEMPLATE_CHANNEL ?= stable
# Image URL to use all building/pushing image targets
IMG_REGISTRY ?= op-kcp-registry:56888/operator-images
IMG ?= $(IMG_REGISTRY)/$(MODULE_NAME)-operator:$(MODULE_VERSION)
```

## Build your operator image

If not done already, first build and push your operator binary by adjusting `IMG`if necessary and then executing the `make module-image` command.

```sh
make module-image
```

This will build the operator image and then push it as the image defined in `IMG`.

## Build and bundle your module

1. Build and push the Module and its descriptor with `module-build`.

```sh
make module-build
```

Now there is a `template.yaml`that you can apply to the Control Plane.

```sh
kubectl apply -f template.yaml
```

2. Verify that the module creation succeeded and observe the `mod` folder. It will contain a `component-descriptor.yaml` with a definition of local layers.

```yaml
component:
  componentReferences: []
  name: kyma.project.io/module/template
  provider: internal
  repositoryContexts:
  - baseUrl: op-kcp-registry:56888/unsigned
    componentNameMapping: urlPath
    type: ociRegistry
  resources:
  - access:
      filename: sha256:4ca3dcc19af77a57e0345018985aec0e7bf15a4fb4ae5b1c5392b45ea013c59a
      mediaType: application/gzip
      type: localFilesystemBlob
    name: template-operator
    relation: local
    type: helm-chart
    version: 0.0.0
  # other layers will be included here
meta:
  schemaVersion: v2
```

As you can see the CLI created various layers that are referenced in the `blobs` directory. For more information on layer structure please reference the module creation with `kyma alpha mod create --help`.

3. Generate a Module Template for installation in the Control Plane.

```sh
make module-template-build
```

## Create another API

If not done already, first install the `kubebuilder`CLI.

```bash
# download kubebuilder and install locally.
curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
```

1. Navigate to the `operator` subproject.

```sh
cd operator
```

2. Create API group version and kind for the intended custom resource(s). Please make sure the `group` is set as `component`.

```
kubebuilder create api --group component --version v1alpha1 --kind Sample
```

3. `kubebuilder` will ask to create Resource, input `y`.
4. `kubebuilder` will ask to create Controller, input `y`.
5. Update go dependencies `go mod tidy`.
6. Run `make generate` followed by `make manifests`, to generate boilerplate code and CRDs respectively.

A basic kubebuilder extension with appropriate scaffolding should be setup.

## Status sub-resource for custom resource(s)

A custom resource is required to contain a specific set of properties in the Status object, to be tracked by the [kyma-operator](https://github.com/kyma-project/kyma-operator/tree/main/operator).
This is required to track the current state of the module, represented by this custom resource.

1. Check reference implementation of the [SampleStatus](./api/v1alpha1/sample_types.go) struct of Sample custom resource. Similarly `.status.state` property of your custom resource *MUST* contain these state values.
On top, `.status` object could contain other relevant properties as per your requirements.
2. Next, check method `Reconcile()` inside the [SampleController](./controllers/sample_controller.go), which demonstrates how `.status.state` properties could be set, depending upon your logic.
3. The `.status.state` value has a literal sense behind it, so use them appropriately.