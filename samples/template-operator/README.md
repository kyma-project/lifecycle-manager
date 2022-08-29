# template-operator
This documentation serves as a reference to implement a module (component) operator, for integration with the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for custom resource definitions (CRDs).
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

## Bundling and installation

### Makefile structure
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
MODULE_REGISTRY ?= kcp-registry.localhost:61370/unsigned
# Desired Channel of the Generated Module Template
MODULE_TEMPLATE_CHANNEL ?= stable
# Image URL to use all building/pushing image targets
IMG_REGISTRY ?= kcp-registry.localhost:61370/operator-images
IMG ?= $(IMG_REGISTRY)/$(MODULE_NAME)-operator:$(MODULE_VERSION)
```

### Build module operator image

Build and push your operator binary by adjusting `IMG`if necessary and then executing the `make module-image` command.
   
```sh
make module-image
```
This will build the operator image and then push it as the image defined in `IMG`.

### Build and push your module to the registry

1. Build and push the module and its descriptor with `module-build`.

   ```sh
   make module-build
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

3. As a result `template.yaml` should be generated in your root folder, that should be applied in the control plane as the source for module configuration.

    ```sh
    kubectl apply -f template.yaml
    ```
    
    You can install the necessary module-template CRD from [here](https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/config/crd/bases/operator.kyma-project.io_moduletemplates.yaml).



## Implementation

### Pre-requisites
* [kubebuilder](https://book.kubebuilder.io/)
    ```bash
    # download kubebuilder and install locally.
    curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
    chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
    ```
* [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Generate kubebuilder operator with kyma module requirements

1. Create a folder and make a directory, e.g. `operator`.
    ```sh
    mkdir operator && cd operator
    ```
   
2. Initialize `kubebuilder` project. Please make sure domain is set to `component.kyma-project.io`.
    ```sh 
   kubebuilder init --domain component.kyma-project.io --repo component.kyma-project.io/test-operator --plugins=go/v4-alpha
    ```

3. Create API group version and kind for the intended custom resource(s). Please make sure the `group` is set as `component`.
    ```
    kubebuilder create api --group component --version v1alpha1 --kind Sample
    ```

4. `kubebuilder` will ask to create Resource, input `y`.

5. `kubebuilder` will ask to create Controller, input `y`.

6. Update go dependencies `go mod tidy`.

7. Run `make generate` followed by `make manifests`, to generate boilerplate code and CRDs respectively.

A basic kubebuilder operator with appropriate scaffolding should be setup.

### Status sub-resource for custom resource(s)

A custom resource is required to contain a specific set of properties in the Status object, to be tracked by the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
This is required to track the current state of the module, represented by this custom resource.

1. Check reference implementation of [Status](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/types/declaritive.go) reference implementation. The `.status.state` field of your custom resource _MUST_ contain one of these state values at all times.
   On top, `.status` object could contain other relevant properties as per your requirements.
2. The `.status.state` values have literal meaning behind them, so use them appropriately.

### Grafana dashboard

By the following command, two grafana dashboard files with controller related metrics will be generated under `/operator/grafana` folder.

```shell
make grafana-dashboard
```

For how to import dashboard, please read [official grafana guide](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard).
This feature is supported by [kubebuilder grafana plugin](https://book.kubebuilder.io/plugins/grafana-v1-alpha.html).

## Abstraction: Reconciliation and Status handling

For simple use cases where a `module operator` should install a `module helm chart(s)` and set the state of the corresponding `module CR` accordingly, a declarative approach is useful.
This approach will enable orchestration of Kubernetes resources so that module owners can concentrate on their specific logic.

### Steps:

1. Refer to [API definition](./operator/api/v1alpha1/sample_types.go) of `SampleCR` and implement `Status` sub-resource similarly.
   This `Status` type definition is sourced from the `module-manager` declarative library and contains all valid `.status.state` values as discussed in the previous sections.
   ```yaml
    Status types.Status `json:"status,omitempty"`
   ```
   
2. Ensure the module CR implements the `module-manager` declarative library's resource requirements, represented by `types.CustomObject`. Also implement missing interface methods.
   ```go
   var _ types.CustomObject = &Sample{}

   func (s *Sample) GetStatus() types.Status {
        return s.Status
   }
   
   func (s *Sample) SetStatus(status types.Status) {
        s.Status = status
   }
   
   func (s *Sample) ComponentName() string {
        return "sample-component-name"
   }
   ```

3. Refer to the [controller implementation](./operator/controllers/sample_controller.go). 
Instead of implementing the default reconciler interface, as provided by `kuberbuilder`, include the `module-manager` declarative reconciler.
   ```go
   // SampleReconciler reconciles a Sample object
   type SampleReconciler struct {
        declarative.ManifestReconciler
        client.Client
        Scheme *runtime.Scheme
        *rest.Config
   }
   ```
   Notice there is not `Reconcile()` method implemented in this controller, since the logic is abstracted within the declarative reconciler.
   
4. As part of reconciler's SetupWithManager() in the Sample CR [controller implementation](./operator/controllers/sample_controller.go), declarative options have been used.
   ```go
   return r.Inject(mgr, &v1alpha1.Sample{},
        declarative.WithManifestResolver(manifestResolver),
        declarative.WithResourceLabels(map[string]string{"sampleKey": "sampleValue"}),
        declarative.WithObjectTransform(transform),
        declarative.WithResourcesReady(true), 
   )
   ```
   These options can be used modify manifest installation and uninstallation. Some options are applied as a manifest pre-processing step and others as post-processing.
   More details on these steps can be found in the [options documentation](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/declarative/options.go).

5. A manadatory requirement of this reconciler is to provide the option `declarative.WithManifestResolver(manifestResolver)`, as it holds the chart information to be processed by the declarative reconciler. 

   This ManifestResolver should implement `types.ManifestResolver` from the declarative library. 
   E.g. Sample CR [controller implementation](./operator/controllers/sample_controller.go) returns chart information.
   ```go
      // Get returns the chart information to be processed.
	  func (m *ManifestResolver) Get(obj types.BaseCustomObject) (types.InstallationSpec, error) {
            sample, valid := obj.(*v1alpha1.Sample)
            if !valid {
                return types.InstallationSpec{},
                fmt.Errorf("invalid type conversion for %s", client.ObjectKeyFromObject(obj))
            }
            return types.InstallationSpec{
                ChartPath:   "./module-chart",
                ReleaseName: sample.Spec.ReleaseName,
                ConfigFlags: map[string]interface{}{
                    "Namespace":       "redis",
                    "CreateNamespace": true,
                },
                SetFlags: map[string]interface{}{
                    "nameOverride": "custom-name-override",
                },
            }, nil
	  }
   ```
6. Run `make generate` followed by `make manifests`, to generate boilerplate code and CRDs respectively.
7. Install your module CR on a cluster and execute `make run` against the cluster's kubeconfig to start your operator locally. 
   If everything is set up properly you should see state changes on your module CR, depending upon chart processing. 