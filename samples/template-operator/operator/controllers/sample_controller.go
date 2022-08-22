/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-operator/samples/template-operator/api/v1alpha1"
	"github.com/kyma-project/manifest-operator/operator/pkg/custom"
	manifestLib "github.com/kyma-project/manifest-operator/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/strvals"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SampleReconciler reconciles a Sample object
type SampleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
}

const deletionFinalizer = "deletion-finalizer"

//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// TODO: dynamically create RBACs! Remove line below.
//+kubebuilder:rbac:groups="*",resources="*",verbs=get;list;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Sample object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *SampleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// TODO(user): your logic here

	// check if Sample resource exists
	sampleResource := &v1alpha1.Sample{}
	if err := r.Get(ctx, req.NamespacedName, sampleResource); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !sampleResource.DeletionTimestamp.IsZero() &&
		sampleResource.Status.State != v1alpha1.SampleStateDeleting {
		// if the status is not yet set to deleting, also update the status
		sampleResource.Status.State = v1alpha1.SampleStateDeleting
		return ctrl.Result{}, r.Status().Update(ctx, sampleResource)
	}

	// add deletion finalizer
	if controllerutil.AddFinalizer(sampleResource, deletionFinalizer) {
		return ctrl.Result{}, r.Client.Update(ctx, sampleResource)
	}

	switch sampleResource.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, sampleResource)
	case v1alpha1.SampleStateProcessing:
		return ctrl.Result{RequeueAfter: time.Second * 3}, r.HandleProcessingState(ctx, sampleResource)
	case v1alpha1.SampleStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx, sampleResource)
	case v1alpha1.SampleStateError:
		return ctrl.Result{RequeueAfter: time.Second * 3}, r.HandleErrorState(ctx, sampleResource)
	case v1alpha1.SampleStateReady:
		return ctrl.Result{RequeueAfter: time.Second * 3}, r.HandleReadyState(ctx, sampleResource)
	}

	return ctrl.Result{}, nil
}

func (r *SampleReconciler) HandleInitialState(ctx context.Context, sampleResource *v1alpha1.Sample) error {
	// TODO: initial logic here

	// Example: Set to Processing state
	sampleResource.Status.State = v1alpha1.SampleStateProcessing
	return r.Client.Status().Update(ctx, sampleResource)
}

func (r *SampleReconciler) HandleProcessingState(ctx context.Context, sampleResource *v1alpha1.Sample) error {
	// TODO: processing logic here
	logger := log.FromContext(ctx)
	manifestClient, err := r.getManifestClient(&logger, sampleResource.Spec.ChartFlags)
	if err != nil {
		sampleResource.Status.State = v1alpha1.SampleStateError
		return r.Client.Status().Update(ctx, sampleResource)
	}

	// Use manifest library client to install a sample chart
	installInfo, err := r.PrepareInstallInfo(ctx, sampleResource)
	if err != nil {
		return err
	}

	ready, err := manifestClient.Install(installInfo)
	if err != nil {
		logger.Error(err, "")
		sampleResource.Status.State = v1alpha1.SampleStateError
		return r.Client.Status().Update(ctx, sampleResource)
	}
	if ready {
		sampleResource.Status.State = v1alpha1.SampleStateReady
		return r.Client.Status().Update(ctx, sampleResource)
	}
	// not ready + no error
	return nil
}

func (r *SampleReconciler) getManifestClient(logger *logr.Logger, configString string,
) (*manifestLib.Operations, error) {
	config := map[string]interface{}{}
	if err := strvals.ParseInto(configString, config); err != nil {
		return nil, err
	}
	// Example: Prepare manifest library client
	return manifestLib.NewOperations(logger, r.Config, "nginx-release-name", cli.New(),
		map[string]map[string]interface{}{
			// check --set flags parameter for helm
			"set": {},
			// comma separated values of manifest command line flags
			"flags": config,
		})
}

func (r *SampleReconciler) HandleDeletingState(ctx context.Context, sampleResource *v1alpha1.Sample) error {
	// TODO: deletion logic here
	logger := log.FromContext(ctx)
	manifestClient, err := r.getManifestClient(&logger, sampleResource.Spec.ChartFlags)
	if err != nil {
		sampleResource.Status.State = v1alpha1.SampleStateError
		return r.Client.Status().Update(ctx, sampleResource)
	}

	// Use manifest library client to install a sample chart
	installInfo, err := r.PrepareInstallInfo(ctx, sampleResource)
	if err != nil {
		return err
	}

	readyToBeDeleted, err := manifestClient.Uninstall(installInfo)
	if err != nil {
		logger.Error(err, "")
		sampleResource.Status.State = v1alpha1.SampleStateError
		return r.Client.Status().Update(ctx, sampleResource)
	}
	if readyToBeDeleted {
		// Example: If Deleting state, remove Finalizers
		sampleResource.Finalizers = nil
		return r.Client.Update(ctx, sampleResource)
	}
	return nil
}

func (r *SampleReconciler) HandleErrorState(ctx context.Context, sampleResource *v1alpha1.Sample) error {
	// TODO: error logic here

	// Example: If Error state, set state to Processing
	sampleResource.Status.State = v1alpha1.SampleStateProcessing
	return r.Client.Status().Update(ctx, sampleResource)
}

func (r *SampleReconciler) HandleReadyState(_ context.Context, _ *v1alpha1.Sample) error {
	// TODO: ready logic here

	// Example: If Ready state, check consistency of deployed module
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Sample{}).
		Complete(r)
}

func (r *SampleReconciler) PrepareInstallInfo(ctx context.Context, sampleResource *v1alpha1.Sample,
) (manifestLib.InstallInfo, error) {
	sampleObjUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sampleResource)
	if err != nil {
		return manifestLib.InstallInfo{}, err
	}
	return manifestLib.InstallInfo{
		Ctx: ctx,
		ChartInfo: &manifestLib.ChartInfo{
			ChartPath:   sampleResource.Spec.ChartPath,
			ReleaseName: sampleResource.Spec.ReleaseName,
		},
		RemoteInfo: custom.RemoteInfo{
			// destination cluster rest config
			RemoteConfig: r.Config,
			// destination cluster rest client
			RemoteClient: &r.Client,
		},
		ResourceInfo: manifestLib.ResourceInfo{
			// base operator resource to be passed for custom checks
			BaseResource: &unstructured.Unstructured{Object: sampleObjUnstructured},
		},
		CheckFn: func(context.Context, *unstructured.Unstructured, *logr.Logger, custom.RemoteInfo) (bool, error) {
			// your custom logic here to set ready state
			return true, nil
		},
		CheckReadyStates: true,
	}, nil
}
