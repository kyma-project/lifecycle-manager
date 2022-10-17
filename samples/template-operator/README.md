# template-operator
This documentation serves as a reference to implement a module (component) operator, for integration with the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for custom resource definitions (CRDs).
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

## Contents
* [Implementation](#implementation)
  * [Pre-requisites](#pre-requisites)
  * [Generate kubebuilder operator](#generate-kubebuilder-operator)
  * [Default (declarative) Reconciliation and Status handling](#default-declarative-reconciliation-and-status-handling)
  * [Custom Reconciliation and Status handling guidelines](#custom-reconciliation-and-status-handling-guidelines)
  * [Local testing](#local-testing)
* [Bundling and installation](#bundling-and-installation)
  * [Makefile structure](#makefile-structure)
  * [Build module operator image](#build-module-operator-image)
  * [Build and push your module to the registry](#build-and-push-your-module-to-the-registry)
* [RBAC](#rbac)
* [Grafana dashboard](#grafana-dashboard)


## Implementation

### Pre-requisites
* k8s cluster
* [kubectl](https://kubernetes.io/docs/tasks/tools/)
* [kubebuilder](https://book.kubebuilder.io/)
    ```bash
    # you could use one of the following options
    
    # option 1: using brew
    brew install kubebuilder
    
    # option 2: fetch sources directly
    curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH)
    chmod +x kubebuilder && mv kubebuilder /usr/local/bin/
    ```


### Generate kubebuilder operator 

1. In your project (module operator) root folder make a directory `operator`
    ```sh
    mkdir operator && cd operator
    ```

2. Initialize `kubebuilder` project. Please make sure domain is set to `kyma-project.io`.
    ```sh 
   kubebuilder init --domain kyma-project.io --repo github.com/kyma-project/test-operator/operator --project-name=test-operator --plugins=go/v4-alpha
    ```

3. Create API group version and kind for the intended custom resource(s). Please make sure the `group` is set as `operator`.
    ```
    kubebuilder create api --group operator --version v1alpha1 --kind Sample
    ```

4. `kubebuilder` will ask to create Resource, input `y`.

5. `kubebuilder` will ask to create Controller, input `y`.

6. Update go dependencies `go mod tidy`.

7. Run `make generate` followed by `make manifests`, to generate boilerplate code and CRDs respectively.

A basic kubebuilder operator with appropriate scaffolding should be setup.

#### Optional: Adjust default config resources
If the module operator will be deployed under same namespace with other operators, differentiate your resources by adding common labels.

1. Add `commonLabels` to default `kustomization.yaml`, [reference implementation](./operator/config/default/kustomization.yaml).

2. Include all resources (e.g: [manager.yaml](./operator/config/manager/manager.yaml)) which contain label selectors by using `commonLabels`.

Further reading: [Kustomize built-in commonLabels](https://github.com/kubernetes-sigs/kustomize/blob/master/api/konfig/builtinpluginconsts/commonlabels.go)
   
### Default (declarative) Reconciliation and Status handling

For simple use cases where a `module operator` should install a `module helm chart(s)` and set the state of the corresponding `module CR` accordingly, a declarative approach is useful.
This approach will enable orchestration of Kubernetes resources so that module owners can concentrate on their specific logic.

#### Steps API definition:

1. Refer to [API definition](./operator/api/v1alpha1/sample_types.go) of `SampleCR` and implement `Status` sub-resource similarly in `./api/<your_api_version>/<cr_name>_types.go`.
   This `Status` type definition is sourced from the `module-manager` declarative library and contains all valid `.status.state` values as discussed in the previous sections.
   ```yaml
    Status types.Status `json:"status,omitempty"`
   ```

2. Ensure the module CR's API definition implements the `module-manager` declarative library's resource requirements, represented by `types.CustomObject`. Also implement missing interface methods.
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

#### Steps controller implementation

1. Refer to the [controller implementation](./operator/controllers/sample_controller.go).
   Instead of implementing the default reconciler interface, as provided by `kubebuilder`, include the `module-manager` declarative reconciler in `./controllers/<cr_name>_controller.go`.
   ```go
   // SampleReconciler reconciles a Sample object
   type SampleReconciler struct {
        declarative.ManifestReconciler // declarative reconciler override
        *rest.Config // required to pass rest config to the declarative library
        client.Client
        Scheme *runtime.Scheme
   }
   ```
   Notice there is no `Reconcile()` method implemented in this controller, since the logic is abstracted within the declarative reconciler.

2. As part of reconciler's `SetupWithManager()` in the Sample CR [controller implementation](./operator/controllers/sample_controller.go), declarative options have been used.
   Similarly, implement the required options in your controller.
   ```go
   return r.Inject(mgr, &v1alpha1.Sample{},
        declarative.WithManifestResolver(manifestResolver),
        declarative.WithCustomResourceLabels(map[string]string{"sampleKey": "sampleValue"}),
        declarative.WithPostRenderTransform(transform),
        declarative.WithResourcesReady(true),
        declarative.WithFinalizer(sampleFinalizer),
   )
   ```
   These options can be used to modify manifest installation and uninstallation. Some options are applied as a manifest pre-processing step and others as post-processing.
   More details on these steps can be found in the [options documentation](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/declarative/options.go).

3. A **mandatory** requirement of this reconciler is to provide the option `declarative.WithManifestResolver(manifestResolver)`, as it holds the chart information to be processed by the declarative reconciler.

   This `ManifestResolver` should implement `types.ManifestResolver` from the declarative library. Implement a similar `ManifestResolver` in your controller.
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
                 ChartPath:   chartPath,
                 ReleaseName: sample.Spec.ReleaseName,
                 ChartFlags: types.ChartFlags{
                    ConfigFlags: types.Flags{
                        "Namespace":       chartNs,
                        "CreateNamespace": true,
                    },
                    SetFlags: types.Flags{
                        "nameOverride": nameOverride,
                    },
                 },
            }, nil
      }
   ```
   
4. Run `make generate`, `make manifests` and in the end `make install`, to generate boilerplate code, CRDs and install required resources on your clusterrespectively.

### Custom Reconciliation and Status handling guidelines

A custom resource is required to contain a specific set of properties in the Status object, to be tracked by the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
This is required to track the current state of the module, represented by this custom resource.

1. Check reference implementation of [Status](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/types/declaritive.go) reference implementation. The `.status.state` field of your custom resource _MUST_ contain one of these state values at all times.
   On top, `.status` object could contain other relevant properties as per your requirements.
2. The `.status.state` values have literal meaning behind them, so use them appropriately.

### Local testing
* Connect to your cluster and ensure `kubectl` is pointing to the desired cluster.
* _Local setup_: install your module CR on a cluster and execute `make run` to start your operator locally.

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

1. Include the module chart represented by `chartPath` from _step 3_ in [Controller implementation](#steps-controller-implementation) above, in your _Dockerfile_.
[Reference implementation](https://github.com/kyma-project/lifecycle-manager/blob/main/samples/template-operator/operator/Dockerfile):
    ```dockerfile
    COPY module-chart/ module-chart/
    ```
2. Adjust the _Dockerfile_ args according to the targeted cluster's architecture and OS

   ```dockerfile
    ARG TARGETOS
    ARG TARGETARCH
   ```

3. Build and push your module operator binary by adjusting `IMG`if necessary and then executing the `make module-image` command.
   
    ```sh
    make module-image
    ```
This will build the operator image and then push it as the image defined in `IMG`.

### Build and push your module to the registry

1. Copy [hack folder](./hack) to your project's root directory

2. The module operator will be packed in a helm chart and pushed to `MODULE_REGISTRY` using `module-build`.

   ```sh
   make module-build
   ```
   
3. Verify that the module creation succeeded and observe the `mod` folder. It will contain a `component-descriptor.yaml` with a definition of local layers.
   
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

4. As a result `template.yaml` should be generated in your root folder, that should be applied in the control plane as the source for module configuration.

    ```sh
    make module-template-push
    ```
    
    You can install the necessary module-template CRD from [here](https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/config/crd/bases/operator.kyma-project.io_moduletemplates.yaml).

## Grafana dashboard

By the following command, two grafana dashboard files with controller related metrics will be generated under `/operator/grafana` folder.

```shell
make grafana-dashboard
```

For how to import dashboard, please read [official grafana guide](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard).
This feature is supported by [kubebuilder grafana plugin](https://book.kubebuilder.io/plugins/grafana-v1-alpha.html).

## RBAC
Make sure you have appropriate authorizations assigned to you controller binary, before you run in inside a cluster.
Sample CR [controller implementation](./operator/controllers/sample_controller.go) includes rbac generation (via kubebuilder) for all resources across all API groups.
This should certainly be adjusted according to the chart manifest resources and reconciliation types.

   ```yaml
      // TODO: dynamically create RBACs! Remove line below.
      //+kubebuilder:rbac:groups="*",resources="*",verbs="*"
   ```
