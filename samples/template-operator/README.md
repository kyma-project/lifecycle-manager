# template-operator
This documentation and template serve as a reference to implement a module (component) operator, for integration with the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for custom resource definitions (CRDs).
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

## Contents
* [Understanding Module Development in Kyma](#understanding-module-development-in-kyma)
* [Implementation](#implementation)
  * [Pre-requisites](#pre-requisites)
  * [Generate kubebuilder operator](#generate-kubebuilder-operator)
  * [Default (declarative) Reconciliation and Status handling](#default-declarative-reconciliation-and-status-handling)
  * [Custom Reconciliation and Status handling guidelines](#custom-reconciliation-and-status-handling-guidelines)
  * [Local testing](#local-testing)
* [Bundling and installation](#bundling-and-installation)
  * [Enhancing a native Makefile of new kubebuilder projects for Module Bundling Workflows](#enhancing-a-native-makefile-of-new-kubebuilder-projects-for-module-bundling-workflows)
  * [Grafana dashboard for simplified Controller Observability](#grafana-dashboard-for-simplified-controller-observability)
  * [RBAC](#rbac)
  * [Build module operator image](#prepare--build-module-operator-image)
  * [Build and push your module to the registry](#build-and-push-your-module-to-the-registry)
* [Using your Module in the Lifecycle Manager Ecosystem](#using-your-module-in-the-lifecycle-manager-ecosystem)
  * [Creating your own Kyma Runtime Custom Resource](#creating-your-own-kyma-runtime-custom-resource)
  * [Debugging the Operator Ecosystem](#debugging-the-operator-ecosystem)
  * [Registering your Module within the Control-Plane](#registering-your-module-within-the-control-plane)

## Understanding Module Development in Kyma 

Before going in-depth, make sure you are familiar with:

- [Modularization in Kyma](https://github.com/kyma-project/community/tree/main/concepts/modularization)
- [Operator Pattern in Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

This Guide serves as comprehensive Step-By-Step tutorial on how to properly create a module from scratch by using an operator that is installing a Helm Chart. 
Note that while other approaches are encouraged, there is no dedicated guide available yet and these will follow with sufficient requests and adoption of Kyma modularization.

Every Kyma Module using an Operator follows 5 basic Principles:

- Declared as available for use in a release channel through the `ModuleTemplate` Custom Resource in the Control-Plane
- Declared as desired State within the `Kyma` Custom Resource in Runtime or Control-Plane
- Installed / Managed in the Runtime by Module-Manager through the `Manifest` Custom Resource in the Control-Plane
- Owns at least 1 CRD that is able to define the Contract that can configure its Behavior
- Is operating on at most 1 Runtime at every given time

Release channels let customers try new modules and features early, and decide when the updates should be applied. For more info, see the [relevant Documentation in our Modularization Overview](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels).

In case you are planning to migrate a pre-existing module within Kyma, please familiarize yourself with the [Transition Plan for existing Modules](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md)

## Implementation

### Pre-requisites

* A provisioned Kubernetes Cluster and OCI Registry

  _WARNING: For all use cases in the guide, you will need a cluster for end-to-end testing outside your envtest Integration Test suite.
  In addition, the default settings used in the guide use standard values coming from our guides for cluster provisioning.
  Thus, we HIGHLY RECOMMEND you should use our [Guide on Cluster and OCI Registry Provisioning for the Operator Infrastructure](../../docs/developer/provision-cluster-and-registry.md) for a smooth development process.
  This is a good alternative if you do not want to use an entire control-plane infrastructure and still want to properly test your operators._
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

1. Initialize `kubebuilder` project. Please make sure domain is set to `kyma-project.io`.
    ```shell
   kubebuilder init --domain kyma-project.io --repo github.com/kyma-project/test-operator/operator --project-name=test-operator --plugins=go/v4-alpha
    ```

2. Create API group version and kind for the intended custom resource(s). Please make sure the `group` is set as `operator`.
    ```shell
    kubebuilder create api --group operator --version v1alpha1 --kind Sample --resource --controller --make
    ```

3. Run `make manifests`, to generate CRDs respectively.

A basic kubebuilder operator with appropriate scaffolding should be setup.

#### Optional: Adjust default config resources
If the module operator will be deployed under same namespace with other operators, differentiate your resources by adding common labels.

1. Add `commonLabels` to default `kustomization.yaml`, [reference implementation](config/default/kustomization.yaml).

2. Include all resources (e.g: [manager.yaml](config/manager/manager.yaml)) which contain label selectors by using `commonLabels`.

Further reading: [Kustomize built-in commonLabels](https://github.com/kubernetes-sigs/kustomize/blob/master/api/konfig/builtinpluginconsts/commonlabels.go)
   
### Default (declarative) Reconciliation and Status handling

_Warning: This declarative approach to reconciliation is inherited from the [kubebuilder declarative pattern](https://github.com/kubernetes-sigs/kubebuilder-declarative-pattern).
It is meant to ease development for newcomers to controller implementations and provides easy to use best-practices for simple use cases. For more complex scenarios, **DO NOT USE** our declarative pattern but build your own reconciliation loop._

For simple use cases where a `module operator` should install a `module helm chart(s)` and set the state of the corresponding `module CR` accordingly, a declarative approach is useful.
This approach will enable orchestration of Kubernetes resources so that module owners can concentrate on their specific logic.

#### Steps API definition:

1. Refer to [API definition](api/v1alpha1/sample_types.go) of `SampleCR` and implement `Status` sub-resource similarly in `./api/<your_api_version>/<cr_name>_types.go`.
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

1. Refer to the [controller implementation](controllers/sample_controller.go).
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

2. As part of reconciler's `SetupWithManager()` in the Sample CR [controller implementation](controllers/sample_controller.go), declarative options have been used.
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
   E.g. Sample CR [controller implementation](controllers/sample_controller.go) returns chart information.
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
   
4. Run `make generate manifests install`, to generate boilerplate code, CRDs and install required resources on your cluster respectively.

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

### Enhanced Makefile structure
The template operator contains base scaffolding that is prepared to build a Kyma Module from the various commands in `Makefile`.

It is a slightly adjusted Makefile, that contains special instructions on how to build and bundle Kyma Modules from
Kubebuilder projects together with the kyma CLI. We highly encourage you get inspired by this Makefile to implement
your own automations as it pushes the entry barrier for building modules much lower.

Here you can see the commands supported with the enhanced Makefile.

```
Usage:
  make <target>

General
  help             Display this help.

Development
  manifests        Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
  generate         Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
  test             Run tests.

Build
  build            Build manager binary.
  run              Run a controller from your host.
  docker-build     Build docker image with the manager.
  docker-push      Push docker image with the manager.

Deployment
  install          Install CRDs into the K8s cluster specified in ~/.kube/config.
  uninstall        Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
  deploy           Deploy controller to the K8s cluster specified in ~/.kube/config.
  undeploy         Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.

Module
  module-image     Build the Module Image and push it to a registry defined in IMG_REGISTRY
  module-build     Build the Module and push it to a registry defined in MODULE_REGISTRY TODO change kyma cli path
  module-template-push  Pushes the ModuleTemplate referencing the Image on MODULE_REGISTRY

Tools
  kustomize        Download & Build kustomize locally if necessary.
  controller-gen   Download & Build controller-gen locally if necessary.
  envtest          Download & Build envtest-setup locally if necessary.
  kyma             Download kyma locally if necessary.

Checks
  fmt              Run go fmt against code.
  vet              Run go vet against code.
  lint             Download & Build & Run golangci-lint against code.
```

### Enhancing a native Makefile of new kubebuilder projects for Module Bundling Workflows

To use your own Operator like Template Operator, you will need to adjust your Makefile with some additional Steps:

Replace the placeholder IMG variable in the Old Makefile with a more sophisticated setup, allowing you to configure not only registry settings for the operator binary,
but also for the Module later on.

1. Replace the naturally generated `IMG` environment variable for the controller image of kubebuilder
    ```makefile
    # Image URL to use all building/pushing image targets
    IMG ?= controller:latest
    ```
    
    with
    
    ```makefile
    # Module Name used for bundling the OCI Image and later on for referencing in the Kyma Modules
    MODULE_NAME ?= template
    # Semantic Module Version used for identifying the build
    MODULE_VERSION ?= 0.0.4
    # Module Registry used for pushing the image
    MODULE_REGISTRY_PORT ?= 8888
    MODULE_REGISTRY ?= op-kcp-registry.localhost:$(MODULE_REGISTRY_PORT)/unsigned
    # Desired Channel of the Generated Module Template
    MODULE_TEMPLATE_CHANNEL ?= stable
    
    # Credentials used for authenticating into the module registry
    # see `kyma alpha mod create --help for more info`
    # MODULE_CREDENTIALS ?= testuser:testpw
    
    # Image URL to use all building/pushing image targets
    IMG_REGISTRY_PORT ?= $(MODULE_REGISTRY_PORT)
    IMG_REGISTRY ?= op-skr-registry.localhost:$(IMG_REGISTRY_PORT)/unsigned/operator-images
    IMG ?= $(IMG_REGISTRY)/$(MODULE_NAME)-operator:$(MODULE_VERSION)
    
    # This will change the flags of the `kyma alpha module create` command in case we spot credentials
    # Otherwise we will assume http-based local registries without authentication (e.g. for k3d)
    ifeq (,$(MODULE_CREDENTIALS))
    MODULE_CREATION_FLAGS=--registry $(MODULE_REGISTRY) -w --insecure
    else
    MODULE_CREATION_FLAGS=--registry $(MODULE_REGISTRY) -w -c $(MODULE_CREDENTIALS)
    endif
    ```

2. Next, create an additional section with commands for module bundling and processing before `##@ Tools`:
    ```makefile
    ##@ Module
    
    .PHONY: module-image
    module-image: docker-build docker-push ## Build the Module Image and push it to a registry defined in IMG_REGISTRY
        echo "built and pushed module image $(IMG)"
    
    .PHONY: module-build
    module-build: kyma ## Build the Module and push it to a registry defined in MODULE_REGISTRY TODO change kyma cli path
        /Users/D067928/SAPDevelop/go/src/github.com/kyma-project/cli/bin/kyma-darwin alpha create module kyma.project.io/module/$(MODULE_NAME) $(MODULE_VERSION) . $(MODULE_CREATION_FLAGS)
    
    .PHONY: module-template-push
    module-template-push: ## Pushes the ModuleTemplate referencing the Image on MODULE_REGISTRY
        kubectl apply -f template.yaml
    ```

3. Last but not least, introduce a new tool dependency that is able to fetch the kyma CLI under `##@ Tools`:
    ```makefile
    ########## Kyma CLI ###########
    KYMA_STABILITY ?= unstable
    
    KYMA ?= $(LOCALBIN)/kyma-$(KYMA_STABILITY)
    kyma: $(KYMA) ## Download kyma locally if necessary.
    $(KYMA): $(LOCALBIN)
        test -f $@ || curl -# -Lo $(KYMA) https://storage.googleapis.com/kyma-cli-$(KYMA_STABILITY)/kyma-darwin
        chmod +x $(KYMA)
    ```

### Grafana dashboard for simplified Controller Observability

You can extend the Operator further by using automated dashboard generation for grafana with this enhancement:

By the following command, two grafana dashboard files with controller related metrics will be generated under `/grafana` folder.

```shell
kubebuilder edit --plugins grafana.kubebuilder.io/v1-alpha
```

For how to import the dashboard, please read [official grafana guide](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard).
This feature is supported by [kubebuilder grafana plugin](https://book.kubebuilder.io/plugins/grafana-v1-alpha.html).

### RBAC
Make sure you have appropriate authorizations assigned to you controller binary, before you run it inside a cluster (not locally with `make run`).
The Sample CR [controller implementation](controllers/sample_controller.go) includes rbac generation (via kubebuilder) for all resources across all API groups.
This should be adjusted according to the chart manifest resources and reconciliation types.

A simple Test RBAC Adjustment can be made to your controller in case you want to delay the engineering of necessary RBACs for your controller to a later stage of development:

```go
package controllers
// TODO: dynamically create RBACs! Remove line below.
//+kubebuilder:rbac:groups="*",resources="*",verbs="*"
```


### Prepare & Build module operator image

_WARNING: This step requires the working OCI Registry from our [Pre-requisites](#pre-requisites)_

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

3. Build and push your module operator binary by adjusting `IMG` if necessary (take a look at the Makefile Preparation in the previous sections for more Details) and then executing the `make module-image` command.
   
    ```sh
    make module-image
    ```
   
This will build the operator image and then push it as the image defined in `IMG`.

### Build and push your module to the registry

_WARNING: This step requires the working OCI Registry and Cluster from our [Pre-requisites](#pre-requisites)_

1. The module operator will be packed in a helm chart and pushed to `MODULE_REGISTRY` using `module-build`.

   ```sh
   make module-build
   ```
   
   In certain cases (e.g. for demos, testing or in CI), it might make sense to run the `ModuleTemplate` in "control-plane" mode.
   What this means is that instead of expecting separate control-plane and runtime clusters, we will target a single cluster that holds all infrastructure for the operators and modules.

   To make a setup work that depends on single-cluster mode you will need to adjust the generated `template.yaml` to install the module in the control-plane.
   To do this edit the field `.spec.target` in the generated `template.yaml` to `control-plane`:

   ```yaml
   apiVersion: operator.kyma-project.io/v1alpha1
   kind: ModuleTemplate
   #...
   spec:
     target: control-plane
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

3. Now run the command for creating the ModuleTemplate in the Cluster.
   After this the module will be available for consumption based on the module name configured with the label `operator.kyma-project.io/module-name` on the ModuleTemplate.
   
   ```sh
   make module-template-push
   ```

   * _WARNING: Depending on your setup against either a k3d cluster/registry, you will need to run the script in `hack/local-template.sh` before pushing the ModuleTemplate to have proper registry setup. (This is necessary for k3d clusters due to port-mapping issues in the cluster that the operators cannot reuse, please take a look at the [relevant issue for more details](https://github.com/kyma-project/module-manager/issues/136#issuecomment-1279542587))_

   * Depending on the state of your target cluster, it can happen that the namespace that the template is in must still be created.

   * You can install the necessary module-template CRD from [here](https://raw.githubusercontent.com/kyma-project/lifecycle-manager/main/operator/config/crd/bases/operator.kyma-project.io_moduletemplates.yaml) if not already installed in your cluster.


## Using your Module in the Lifecycle Manager Ecosystem

### Creating your own Kyma Runtime Custom Resource

_WARNING: This step requires the working OCI Registry and Cluster from our [Pre-requisites](#pre-requisites)_

Now that everything is prepared in a cluster of your choosing, you are free to reference the module within any `Kyma` Custom Resource in the Cluster that you prepared earlier.

For our Template-Operator, you could generate a Kyma Resource in `kyma.yaml` with this:

```shell
cat <<EOF > kyma.yaml
apiVersion: operator.kyma-project.io/v1alpha1
kind: Kyma
metadata:
  name: kyma-sample
  namespace: $(yq '.metadata.namespace' template.yaml)
spec:
  channel: $(yq '.spec.channel' template.yaml)
  sync:
    enabled: false
  modules:
    - name: $(yq '.metadata.labels | with_entries(select(.key == "operator.kyma-project.io/module-name")) | .[]' template.yaml)
EOF
```

Note that of course, you can adjust the Kyma CR based on your testing scenario. For example, if you are running a Dual-Cluster Setup, you might want to enable the synchronization of the Kyma Resource into the Runtime for E2E configurability.

To get started with the installation, simply run `kubectl apply -f kyma.yaml` and the installation should start almost immediately.

### Debugging the Operator Ecosystem

Of course, the operator ecosystem around Kyma is highly complex. So complex in fact, that it might become troublesome to debug issues in case your module is not installed.
For this very reason here is a small help to debug any module developed via this guide.

1. Verify the Kyma Installation state is ready by verifying all conditions
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}' \
    && kubectl get kyma -o jsonpath="$JSONPATH" | grep "Ready=True"
   ```
2. Verify the Manifest Installation state is ready by verifying all conditions
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}' \
    && kubectl get manifest -o jsonpath="$JSONPATH" | grep "Ready=True"
   ```
3. Depending on your issue, either observe the Deployment logs from either `lifecycle-manager` and/or `module-manager`. Make sure that no errors occur.

Usually the issue is related to either RBAC Configuration (for troubleshooting minimum privileges for the controllers, see our dedicated [RBAC](#rbac) section), or a misconfiguration of the `ModuleTemplate`.
In the later case, make sure that you are aware if you are running within a single cluster or with a separate control-plane, and watch out for any Steps with `WARNING` attached to them and retry with a freshly provisioned cluster.
For cluster provisioning, please make sure to follow our recommendations for Clusters mentioned in our [Pre-requisites](#pre-requisites) for this guide.

Lastly, if you are still unsure, please feel free to open an Issue and describe what's going on, and we will be happy to help you out with more detailed information.

### Registering your Module within the Control-Plane

For global usage of your module, the generated `template.yaml` from [Build and push your module to the registry](#build-and-push-your-module-to-the-registry) needs to be registered in our control-plane.
This relates to [Phase 2 of the Module Transition Plane](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md#phase-2---first-module-managed-by-kyma-operator-integrated-with-keb). Please be patient until we can provide you with a stable guide on how to properly integrate your template.yaml
with an automated test flow into the central Control-Plane Offering.