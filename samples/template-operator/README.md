# Template Operator
This documentation and template serve as a reference to implement a module (component) operator, for integration with the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
It utilizes the [kubebuilder](https://book.kubebuilder.io/) framework with some modifications to implement Kubernetes APIs for custom resource definitions (CRDs).
Additionally, it hides Kubernetes boilerplate code to develop fast and efficient control loops in Go.

## Contents
* [Understanding module development in Kyma](#understanding-module-development-in-kyma)
* [Implementation](#implementation)
  * [Pre-requisites](#pre-requisites)
  * [Generate kubebuilder operator](#generate-kubebuilder-operator)
  * [Default (declarative) reconciliation and status handling](#default-declarative-reconciliation-and-status-handling)
  * [Custom reconciliation and status handling guidelines](#custom-reconciliation-and-status-handling-guidelines)
  * [Local testing](#local-testing)
* [Bundling and installation](#bundling-and-installation)
  * [Grafana dashboard for simplified Controller Observability](#grafana-dashboard-for-simplified-controller-observability)
  * [RBAC](#rbac)
  * [Build module operator image](#prepare--build-module-operator-image)
  * [Build and push your module to the registry](#build-and-push-your-module-to-the-registry)
* [Using your module in the Lifecycle Manager ecosystem](#using-your-module-in-the-lifecycle-manager-ecosystem)
  * [Deploying Kyma infrastructure operators with `kyma alpha deploy`](#deploying-kyma-infrastructure-operators-with-kyma-alpha-deploy)
  * [Deploying a `ModuleTemplate` into the Control Plane](#deploying-a-moduletemplate-into-the-control-plane)
  * [Debugging the operator ecosystem](#debugging-the-operator-ecosystem)
  * [Registering your module within the cCntrol Plane](#registering-your-module-within-the-control-plane)

## Understanding module development in Kyma 

Before going in-depth, make sure you are familiar with:

- [Modularization in Kyma](https://github.com/kyma-project/community/tree/main/concepts/modularization)
- [Operator Pattern in Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

This Guide serves as comprehensive Step-By-Step tutorial on how to properly create a module from scratch by using an operator that is installing a Helm Chart. 
Note that while other approaches are encouraged, there is no dedicated guide available yet and these will follow with sufficient requests and adoption of Kyma modularization.

Every Kyma Module using an Operator follows 5 basic Principles:

- Declared as available for use in a release channel through the `ModuleTemplate` Custom Resource in the control-plane
- Declared as desired state within the `Kyma` Custom Resource in runtime or control-plane
- Installed / managed in the runtime by [Module-Manager](https://github.com/kyma-project/module-manager/tree/main/) through a `Manifest` custom resource in the control-plane
- Owns at least 1 Custom Resource Definition that defines the contract and configures its behaviour
- Is operating on at most 1 runtime at every given time

Release channels let customers try new modules and features early, and decide when the updates should be applied. For more info, see the [release channels documentation in our Modularization overview](https://github.com/kyma-project/community/tree/main/concepts/modularization#release-channels).

In case you are planning to migrate a pre-existing module within Kyma, please familiarize yourself with the [transition plan for existing modules](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md)

## Implementation

### Pre-requisites

* A provisioned Kubernetes Cluster and OCI Registry

  _WARNING: For all use cases in the guide, you will need a cluster for end-to-end testing outside your [envtest](https://book.kubebuilder.io/reference/envtest.html) integration test suite.
  In addition, the default settings used in this guide are taken over from our [cluster and OCI registry provisioning guide](../../docs/developer/provision-cluster-and-registry.md).
  This guide is HIGHLY RECOMMENDED to be followed for a smooth development process.
  This is a good alternative if you do not want to use an entire control-plane infrastructure and still want to properly test your operators.__
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
* [kyma CLI](https://github.com/kyma-project/cli#installation)
* A HELM Chart to install from your control-loop (if you do not have one ready, feel free to use the stateless redis chart from this sample)

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
It eases the controller implementation effort of developers, covering simple use cases based on best practices . For more complex scenarios, **DO NOT USE** our declarative pattern but build your own reconciliation loop instead.__

For simple use cases where an operator should install one or many helm chart(s) and set the state of the corresponding Custom Resource accordingly, a declarative approach is useful for abstracting (sometimes heavy) boilerplate.
This approach will enable orchestration of Kubernetes resources so that module owners can concentrate on their specific logic.

To make use of our declarative library, simply import it with

```shell
go get github.com/kyma-project/module-manager/operator@latest
```

#### Steps API definition:

1. Refer to [API definition](api/v1alpha1/sample_types.go) of `SampleCR` and implement `Status` sub-resource similarly in `./api/<your_api_version>/<cr_name>_types.go`.
   This `Status` type definition is sourced from the `module-manager` declarative library and contains all valid `.status.state` values as discussed in the previous sections.
   You can embed it into your existing status object:
   ```go
    package v1alpha1
    import (
        "github.com/kyma-project/module-manager/operator/pkg/types"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    )
    // Sample is the Schema for the samples API
    type Sample struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
    
        Spec   SampleSpec   `json:"spec,omitempty"`
        Status types.Status `json:"status,omitempty"`
    }
   ```

2. Ensure the module CR's API definition implements the `module-manager` declarative library's resource `.status` interface requirements, represented by `types.CustomObject`. Also implement missing interface methods.
   ```go
    package v1alpha1
    import (
        "github.com/kyma-project/module-manager/operator/pkg/types"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    )
    // Sample is the Schema for the samples API
    type Sample struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`
    
        Spec   SampleSpec   `json:"spec,omitempty"`
        Status types.Status `json:"status,omitempty"`
    }
    func (in *Sample) GetStatus() types.Status {
        return in.Status
    }
    
    func (in *Sample) SetStatus(status types.Status) {
        in.Status = status
    }
    
    func (in *Sample) ComponentName() string {
        return "sample-component-name"
    }
   ```

#### Steps controller implementation

1. Refer to the [controller implementation](controllers/sample_controller.go).
   Instead of implementing the default reconciler interface, as provided by `kubebuilder`, include the `module-manager` declarative reconciler in `./controllers/<cr_name>_controller.go`.
    ```go
    package controllers

    import (
	    "github.com/kyma-project/module-manager/operator/pkg/declarative"
	    "sigs.k8s.io/controller-runtime/pkg/client"
	    "k8s.io/apimachinery/pkg/runtime"
    )

    // SampleReconciler reconciles a Sample object
    type SampleReconciler struct {
        declarative.ManifestReconciler // this handles declarative manifest reconciliation
        client.Client
        Scheme *runtime.Scheme
    }
    ```
   _WARNING: Notice there is no `Reconcile()` method implemented in our referenced controller, since the logic is abstracted within the declarative reconciler.
   If you add `declarative.ManifestReconciler` into a plain controller, the declarative `Reconcile` method would be overwritten, so make sure to delete the old method in case you want to use the default logic._
   
   Now, you will still be left with some steps to make our reconciler run with our declarative setup.

2. As part of the controller's `SetupWithManager()` in the Sample CR [controller implementation](controllers/sample_controller.go), we now have to tell the declarative reconciler how to reconcile the object.
   For this, we need to inject the necessary information about the declarative intention on what to reconcile with `Inject(...)`.

   In its most simple form, the new setup could look like this (assuming all the settings from above were used):
    
   <details>
        <summary>Sample implementation</summary>
   
   ```go
    package controllers
    import (
	    "github.com/kyma-project/module-manager/operator/pkg/declarative"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	    "sigs.k8s.io/controller-runtime/pkg/client"
	    ctrl "sigs.k8s.io/controller-runtime"
	    "k8s.io/apimachinery/pkg/runtime"
   
	    operatorv1alpha1 "github.com/kyma-project/test-operator/operator/api/v1alpha1"
    )
   
    // SampleReconciler reconciles a Sample object
    type SampleReconciler struct {
      declarative.ManifestReconciler
      client.Client
      Scheme *runtime.Scheme
    }
   
    // SetupWithManager sets up the controller with the Manager.
    func (r *SampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
    if err := r.Inject(
      mgr, &operatorv1alpha1.Sample{},
      declarative.WithCustomResourceLabels(map[string]string{"sampleKey": "sampleValue"}),
      declarative.WithResourcesReady(true),
      declarative.WithFinalizer("sample-finalizer"), 
    ); err != nil {
      return err
    }
    
    return ctrl.NewControllerManagedBy(mgr).
	    For(&operatorv1alpha1.Sample{}).
	    Complete(r)
    }
   ```
   
   </details>
   
   Some options offered by the declarative library are applied as a manifest pre-processing step and others as post-processing.
   More details on these steps can be found in the [options documentation](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/declarative/options.go) or in the reference implementation.

3. A **mandatory** requirement of this reconciler is to provide the source of chart installation. This can be described using either of these options:

   * `declarative.WithManifestResolver(manifestResolver)`: `Get()` of `types.ManifestResolver` must return an object of `types.InstallationSpec` type. Using this option you can add custom logic in `Get()` additionally. 
   * `declarative.WithDefaultResolver(manifestResolver)`: Sample CR's `.spec` field should implement `types.InstallationSpec` type

   _WARNING: At this point in time, we will assume you have a chart that you want to install with your operator ready under `./module-chart`. If not already done, copy your charts from the [Pre-requisites](#pre-requisites)_

   In both options described  above, `types.InstallationSpec` is implemented, which must contain the necessary fields for chart installation.
 
   E.g. Sample CR [controller implementation](controllers/sample_controller.go) returns chart information for a stateless redis installation. 

   A sample implementation of  `types.ManifestResolver` installing a chart from `./module-chart` in namespace `default` with the `--set` flag `nameOverride=custom-name-override` could look like this:

   <details>
   
    <summary>Sample implementation</summary>
   
    ```go
    package controllers
    import (
       "fmt"
       "github.com/go-logr/logr"
       "github.com/kyma-project/module-manager/operator/pkg/declarative"
       "github.com/kyma-project/module-manager/operator/pkg/types"
       metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
       "sigs.k8s.io/controller-runtime/pkg/client"
       ctrl "sigs.k8s.io/controller-runtime"
       "k8s.io/apimachinery/pkg/runtime"
    
       operatorv1alpha1 "github.com/kyma-project/test-operator/operator/api/v1alpha1"
    )
    var defaultResolver = &ManifestResolver{
       chartPath: "./module-chart",
       configFlags: types.Flags{
         "Namespace":       "redis", // can be omitted if namespace is pre-existing
         "CreateNamespace": true, // can be omitted if namespace is pre-existing
       },
       setFlags: types.Flags{
           "nameOverride": "custom-name-override",
       },
    }
    
    // ManifestResolver represents the chart information for the passed Sample resource.
    type ManifestResolver struct {
       chartPath   string
       setFlags    types.Flags
       configFlags types.Flags
    }
    
    // Get returns the chart information to be processed.
    func (m *ManifestResolver) Get(obj types.BaseCustomObject, _ logr.Logger) (types.InstallationSpec, error) {
       sample, valid := obj.(*operatorv1alpha1.Sample)
       if !valid {
           return types.InstallationSpec{},
               fmt.Errorf("invalid type conversion for %s", client.ObjectKeyFromObject(obj))
       }
       return types.InstallationSpec{
           ChartPath:   m.chartPath,
           ReleaseName: sample.Spec.Foo,
           ChartFlags: types.ChartFlags{
               ConfigFlags: m.configFlags,
               SetFlags:    m.setFlags,
           },
       }, nil
    }
    
    // SampleReconciler reconciles a Sample object
    type SampleReconciler struct {
     declarative.ManifestReconciler
     client.Client
     Scheme *runtime.Scheme
    }
    
    // SetupWithManager sets up the controller with the Manager.
    func (r *SampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
    if err := r.Inject(
     mgr, &operatorv1alpha1.Sample{},
     // Note that now, we need to add it to the Injection so that it's picked up by the declarative reconciler with `declarative.WithManifestResolver(defaultResolver)`
     declarative.WithManifestResolver(defaultResolver),
     declarative.WithCustomResourceLabels(map[string]string{"sampleKey": "sampleValue"}),
     declarative.WithResourcesReady(true),
     declarative.WithFinalizer("sample-finalizer"), 
    ); err != nil {
     return err
    }
    
    return ctrl.NewControllerManagedBy(mgr).
       For(&operatorv1alpha1.Sample{}).
       Complete(r)
    }
    ```
   
   </details>
   
4. Run `make generate manifests`, to generate boilerplate code and manifests.

### Custom reconciliation and status handling guidelines

A custom resource is required to contain a specific set of properties in the Status object, to be tracked by the [lifecycle-manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator).
This is required to track the current state of the module, represented by this custom resource.

1. Check the reference implementation of [Status](https://github.com/kyma-project/module-manager/blob/main/operator/pkg/types/declaritive.go) reference implementation. The `.status.state` field of your custom resource _MUST_ contain one of these state values at all times.
   On top, `.status` object could contain other relevant properties as per your requirements.
2. The `.status.state` values have literal meaning behind them, so use them appropriately.

In case you choose to not use the declarative option (as described in [this step](#default-declarative-reconciliation-and-status-handling)), you can use this contract as a base for your own state reconciliation.
Note however, that you need to be careful in designing your reconciliation loop, and we recommend getting started with our declarative pattern first.

### Local testing
* Connect to your cluster and ensure `kubectl` is pointing to the desired cluster.
* Install CRDs with `make install`
  _WARNING: This installs a CRD on your cluster, so create your cluster before running the `install` command. See [Pre-requisites](#pre-requisites) for details on the cluster setup._
* _Local setup_: install your module CR on a cluster and execute `make run` to start your operator locally.

_WARNING: Note that while `make run` fully runs your controller against the cluster, it is not feasible to compare it to a productive operator.
This is mainly because it runs with a client configured with privileges derived from your `KUBECONFIG` environment variable. For in-cluster configuration, see our [Guide on RBAC Management](#rbac)._

## Bundling and installation

### Grafana dashboard for simplified Controller Observability

You can extend the operator further by using automated dashboard generation for grafana.

By the following command, two grafana dashboard files with controller related metrics will be generated under `/grafana` folder.

```shell
kubebuilder edit --plugins grafana.kubebuilder.io/v1-alpha
```

To import the grafana dashboard, please read [official grafana guide](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard).
This feature is supported by [kubebuilder grafana plugin](https://book.kubebuilder.io/plugins/grafana-v1-alpha.html).

### RBAC
Make sure you have appropriate authorizations assigned to you controller binary, before you run it inside a cluster (not locally with `make run`).
The Sample CR [controller implementation](controllers/sample_controller.go) includes rbac generation (via kubebuilder) for all resources across all API groups.
This should be adjusted according to the chart manifest resources and reconciliation types.

Towards the earlier stages of your operator development RBACs could simply accommodate all resource types and adjusted later, as per your requirements.

```go
package controllers
// TODO: dynamically create RBACs! Remove line below.
//+kubebuilder:rbac:groups="*",resources="*",verbs="*"
```

_WARNING: Do not forget to run `make manifests` after this adjustment for it to take effect!_

### Prepare and build module operator image

_WARNING: This step requires the working OCI Registry from our [Pre-requisites](#pre-requisites)_

1. Include the module chart represented by `chartPath` from _step 3_ in [Controller implementation](#steps-controller-implementation) above, in your _Dockerfile_.
[Reference implementation](https://github.com/kyma-project/lifecycle-manager/blob/main/samples/template-operator/operator/Dockerfile):
    ```dockerfile
    FROM gcr.io/distroless/static:nonroot
    WORKDIR /
    COPY module-chart/ module-chart/
    COPY --from=builder /workspace/manager .
    USER 65532:65532
    
    ENTRYPOINT ["/manager"]
    ``` 

2. Build and push your module operator binary by adjusting `IMG` if necessary and running the inbuilt kubebuilder commands.
   Assuming your operator image has the following base settings:
   * hosted at `op-kcp-registry.localhost:8888/unsigned/operator-images` 
   * controller image name is `sample-operator`
   * controller image has version `0.0.1`

   You can run the following command
    ```sh
    make docker-build docker-push IMG="op-kcp-registry.localhost:8888/unsigned/operator-images/sample-operator:0.0.1"
    ```
   
This will build the controller image and then push it as the image defined in `IMG` based on the kubebuilder targets.

### Build and push your module to the registry

_WARNING: This step requires the working OCI Registry, Cluster and Kyma CLI from our [Pre-requisites](#pre-requisites)_

1. The module operator manifests from the `default` kustomization (not the controller image) will now be bundled and pushed.
   Assuming the settings from [Prepare and build module operator image](#prepare-and-build-module-operator-image) for single-cluster mode, and assuming the following module settings:
   * hosted at `op-kcp-registry.localhost:8888/unsigned`
   * generated for channel `stable`
   * module has version `0.0.1`
   * module name is `template`
   * for a k3d registry enable the `insecure` flag (`http` instead of `https` for registry communication)
   * uses Kyma CLI in `$PATH` under `kyma`
   * a simple `config.yaml` is present for module configuration with the content

     ```yaml
     # Samples Config
     configs:
     ```
     _WARNING: Even though this file is empty, it is mandatory for the command to succeed as it will be bundled as layer!
     kubebuilder projects by default to not have such a file (it is introduced by modularization) and you will need to create one on your own if not already done._ 
   * the default sample under `config/samples/operator_v1alpha1_sample.yaml` has been adjusted to be a valid CR by setting the default generated `Foo` field instead of a TODO.

     ```yaml
     apiVersion: operator.kyma-project.io/v1alpha1
     kind: Sample
     metadata:
       name: sample-sample
     spec:
       foo: bar
     ```
     _WARNING: The settings above reflect your default configuration for a module. If you want to change this you will have to manually adjust it to a different configuration. 
     You can also define multiple files in `config/samples`, however you will need to then specify the correct file during bundling._
   * The `.gitignore` has been adjusted and following ignores were added
   
     ```gitignore
     # kyma module cache
     mod
     # generated dummy charts
     charts
     # kyma generated by scripts or local testing
     kyma.yaml
     # template generated by kyma create module
     template.yaml
     ```

   Now, run the following command to create and push your module operator image to the specified registry:

   ```sh
   kyma alpha create module kyma-project.io/module/sample 0.0.1 . -w --insecure --registry op-kcp-registry.localhost:8888/unsigned
   ```
   
   _WARNING: For external registries (e.g. Google Container/Artifact Registry), never use insecure. Instead specify credentials. More details can be found in the help documentation of the CLI_

   To make a setup work in single-cluster mode adjust the generated `template.yaml` to install the module in the Control Plane, by assigning the field `.spec.target` to value `control-plane`. This will install all operators and modules in the same cluster.

   ```yaml
   apiVersion: operator.kyma-project.io/v1alpha1
   kind: ModuleTemplate
   #...
   spec:
     target: control-plane
   ```

2. Verify that the module creation succeeded and observe the `mod` folder. It will contain a `component-descriptor.yaml` with a definition of local layers.
    
    <details>
        <summary>Sample</summary>    

    ```yaml
    component:
      componentReferences: []
      name: kyma-project.io/module/sample
      provider: internal
      repositoryContexts:
      - baseUrl: op-kcp-registry.localhost:8888/unsigned
        componentNameMapping: urlPath
        type: ociRegistry
      resources:
      - access:
          filename: sha256:fafc3be538f68a786f3b8ef39bd741805314253f81cf4a5880395dcecf599ef5
          mediaType: application/gzip
          type: localFilesystemBlob
        name: sample-operator
        relation: local
        type: helm-chart
        version: 0.0.1
      - access:
          filename: sha256:db86408caca4c94250d8291aa79655b84146f9cc45e0da49f05a52b3722d74a0
          mediaType: application/octet-stream
          type: localFilesystemBlob
        name: config
        relation: local
        type: yaml
        version: 0.0.1
      sources: []
      version: 0.0.1
    meta:
      schemaVersion: v2
    ```
   
    As you can see the CLI created various layers that are referenced in the `blobs` directory. For more information on layer structure please reference the module creation with `kyma alpha mod create --help`.

    </details>

## Using your module in the Lifecycle Manager ecosystem

### Deploying Kyma infrastructure operators with `kyma alpha deploy`

_WARNING: This step requires the working OCI Registry and Cluster from our [Pre-requisites](#pre-requisites)_

Now that everything is prepared in a cluster of your choice, you are free to reference the module within any `Kyma` custom resource in your Control Plane cluster.

Deploy the [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/tree/main/operator) & [Module Manager](https://github.com/kyma-project/module-manager/tree/main/operator) to the Control Plane cluster with:

```shell
kyma alpha deploy
```

_WARNING: For single-cluster mode, module manager needs additional privileges to work in the cluster as it usually does not need to access all resource types within the control-plane.
This can be fixed by editing the necessary ClusterRole with `kubectl edit clusterrole module-manager-manager-role` with the following adjustment:_
```yaml
- apiGroups:                                                                                                                                                  
  - "*"                                                                                                                                                       
  resources:                                                                                                                                                  
  - "*"                                                                                                                                                       
  verbs:                                                                                                                                                      
  - "*"
```

_Note that this is very hard to properly protect against privilege escalation in single-cluster mode, which is one of the reasons we heavily discourage it for productive use_

### Deploying a `ModuleTemplate` into the Control Plane

Now run the command for creating the `ModuleTemplate` in the cluster.
After this the module will be available for consumption based on the module name configured with the label `operator.kyma-project.io/module-name` on the `ModuleTemplate`.

_WARNING: Depending on your setup against either a k3d cluster/registry, you will need to run the script in `hack/local-template.sh` before pushing the ModuleTemplate to have proper registry setup.
(This is necessary for k3d clusters due to port-mapping issues in the cluster that the operators cannot reuse, please take a look at the [relevant issue for more details](https://github.com/kyma-project/module-manager/issues/136#issuecomment-1279542587))_

```sh
kubectl apply -f template.yaml
```

For single-cluster mode, you could use the existing Kyma custom resource generated for the Control Plane in `kyma.yaml` with this:

```shell
kubectl patch kyma default-kyma -n kcp-system --type='json' -p='[{"op": "add", "path": "/spec/modules", "value": [{"name": "sample" }] }]'
```

This adds your module into `.spec.modules` with a name originally based on the `"operator.kyma-project.io/module-name": "sample"` label that was generated in `template.yaml`:

```yaml
spec:
  modules:
  - name: sample
```

If required, you can adjust this Kyma CR based on your testing scenario. For example, if you are running a dual-cluster setup, you might want to enable the synchronization of the Kyma CR into the runtime cluster for a full E2E setup.
On creation of this kyma CR in your Control Plane cluster, installation of the specified modules should start immediately.

### Debugging the operator ecosystem

The operator ecosystem around Kyma is complex, and it might become troublesome to debug issues in case your module is not installed correctly.
For this very reason here are some best practices on how to debug modules developed through this guide.

1. Verify the Kyma installation state is ready by verifying all conditions.
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.reason}:{@.status};{end}{end}' \
    && kubectl get kyma -o jsonpath="$JSONPATH" -n kcp-system
   ```
2. Verify the Manifest installation state is ready by verifying all conditions.
   ```shell
    JSONPATH='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{end}' \
    && kubectl get manifest -o jsonpath="$JSONPATH"-n kcp-system
   ```
3. Depending on your issue, either observe the deployment logs from either `lifecycle-manager` and/or `module-manager`. Make sure that no errors have occurred.

Usually the issue is related to either RBAC configuration (for troubleshooting minimum privileges for the controllers, see our dedicated [RBAC](#rbac) section), mis-configured image, module registry or `ModuleTemplate`.
As a last resort, make sure that you are aware if you are running within a single-cluster or a dual-cluster setup, watch out for any steps with a `WARNING` specified and retry with a freshly provisioned cluster.
For cluster provisioning, please make sure to follow our recommendations for clusters mentioned in our [Pre-requisites](#pre-requisites) for this guide.

Lastly, if you are still unsure, please feel free to open an issue, with a description and steps to reproduce, and we will be happy to help you with a solution.

### Registering your module within the Control Plane

For global usage of your module, the generated `template.yaml` from [Build and push your module to the registry](#build-and-push-your-module-to-the-registry) needs to be registered in our control-plane.
This relates to [Phase 2 of the Module Transition Plane](https://github.com/kyma-project/community/blob/main/concepts/modularization/transition.md#phase-2---first-module-managed-by-kyma-operator-integrated-with-keb). Please be patient until we can provide you with a stable guide on how to properly integrate your template.yaml
with an automated test flow into the central control-plane Offering.