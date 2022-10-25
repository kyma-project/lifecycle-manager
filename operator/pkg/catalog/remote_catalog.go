package catalog

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	remotecontext "github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

type Settings struct {
	SSAPatchOptions *client.PatchOptions
}

type RemoteCatalog struct {
	runtimeClient      client.Client
	controlPlaneClient client.Client
	settings           Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1alpha1.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

// NewRemoteCatalog uses 2 Clients from a Sync Context to create a Catalog in a remote Cluster.
func NewRemoteCatalog(
	ctx *remotecontext.KymaSynchronizationContext,
	settings Settings,
) *RemoteCatalog {
	return &RemoteCatalog{runtimeClient: ctx.RuntimeClient, controlPlaneClient: ctx.ControlPlaneClient, settings: settings}
}

// CreateOrUpdate first lists all currently available moduleTemplates in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD
// After the list has been aggregated from the client, it calculates a 2 stage diff
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
// For more details on when a ModuleTemplate is updated, see CalculateDiffs.
func (c *RemoteCatalog) CreateOrUpdate(
	ctx context.Context,
	moduleTemplatesControlPlane *v1alpha1.ModuleTemplateList,
) error {
	moduleTemplatesRuntime := &v1alpha1.ModuleTemplateList{}
	err := c.runtimeClient.List(ctx, moduleTemplatesRuntime)

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster, then we create it
	if meta.IsNoMatchError(err) {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1alpha1.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
		err = c.runtimeClient.List(ctx, moduleTemplatesRuntime)
	}

	if err != nil {
		return err
	}

	diffApply, diffDelete := c.CalculateDiffs(moduleTemplatesRuntime, moduleTemplatesControlPlane)

	for _, diff := range diffApply {
		diff.SetLastSync()

		var buf bytes.Buffer

		if err := json.NewEncoder(&buf).Encode(diff); err != nil {
			return err
		}
		patch := client.RawPatch(types.ApplyPatchType, buf.Bytes())

		if err := c.runtimeClient.Patch(
			ctx, diff, patch, c.settings.SSAPatchOptions,
		); err != nil {
			return fmt.Errorf("could not apply module template diff: %w", err)
		}
	}

	for _, diff := range diffDelete {
		if err := c.runtimeClient.Delete(ctx, diff); err != nil {
			return fmt.Errorf("could not delete module template from diff: %w", err)
		}
	}
	return nil
}

// CalculateDiffs takes two ModuleTemplateLists and a given Force Interval and produces 2 Pointer Lists
// The first pointer list references all Templates that would need to be applied to the runtime with SSA
// The second pointer list references all Templates that would need to be deleted from the runtime.
//
// By default, a template is deemed as necessary for apply with SSA to the runtime when
// 1. it does not exist in the controlPlane
// 2. it exists but has a mismatching generation in the control-plane (the control plane spec got updated)
// 3. it exists but has a mismatching generation in the runtime (the runtime spec got updated).
//
// The Diff of the Spec is tracked with two annotations.
// A change in the remote spec advances the last sync gen of the remote
// to the remote generation + 1 during the diff since the expected apply
// would increment the generation.
// This saves an additional API Server call to update the generation in the annotation
// as we already know the spec will change. It also saves us the use of any special status field.
// If for some reason the generation is incremented multiple times in between the current and the next reconciliation
// it can simply jump multiple generations by always basing it on the latest generation of the remote.
func (*RemoteCatalog) CalculateDiffs(
	runtimeList *v1alpha1.ModuleTemplateList, controlPlaneList *v1alpha1.ModuleTemplateList,
) ([]*v1alpha1.ModuleTemplate, []*v1alpha1.ModuleTemplate) {
	// these are various ModuleTemplate references which we will either have to create, update or delete from
	// the remote
	var diffToApply []*v1alpha1.ModuleTemplate
	var diffToDelete []*v1alpha1.ModuleTemplate

	// now lets start using two frequency maps to discover diffs
	existingOnRemote := make(map[string]int)
	existingOnControlPlane := make(map[string]int)

	for i := range runtimeList.Items {
		remote := &runtimeList.Items[i]
		existingOnRemote[remote.Namespace+remote.Name] = i
	}

	for i := range controlPlaneList.Items {
		controlPlane := &controlPlaneList.Items[i]
		existingOnControlPlane[controlPlane.Namespace+controlPlane.Name] = i
		// if the controlPlane Template does not exist in the remote, we already know we need to create it
		// in the runtime
		if _, exists := existingOnRemote[controlPlane.Namespace+controlPlane.Name]; !exists {
			prepareControlPlaneTemplateForRuntime(controlPlane)

			diffToApply = append(diffToApply, controlPlane)
		}
	}

	for i := range runtimeList.Items {
		remote := &runtimeList.Items[i]
		controlPlaneIndex, exists := existingOnControlPlane[remote.Namespace+remote.Name]

		// if the remote Template does not exist in the control plane, we already know we need to delete it
		if !exists {
			diffToDelete = append(diffToDelete, remote)
			continue
		}
		remote.ObjectMeta.SetManagedFields([]metav1.ManagedFieldsEntry{})

		if remote.Annotations == nil {
			remote.Annotations = make(map[string]string)
		}

		// if there is a template in controlPlane and remote, but the generation is outdated, we need to
		// update it
		controlPlaneSyncedGen, _ := strconv.Atoi(remote.Annotations[v1alpha1.LastSyncGenerationControlPlane])
		controlPlaneGen := (&controlPlaneList.Items[controlPlaneIndex]).GetGeneration()
		runtimeSyncedGen, _ := strconv.Atoi(remote.Annotations[v1alpha1.LastSyncGenerationRuntime])

		if int64(controlPlaneSyncedGen) != controlPlaneGen {
			remote.Annotations[v1alpha1.LastSyncGenerationControlPlane] = strconv.Itoa(int(controlPlaneGen))
			(&controlPlaneList.Items[controlPlaneIndex]).Spec.DeepCopyInto(&remote.Spec)
			diffToApply = append(diffToApply, remote)
		} else if int64(runtimeSyncedGen) != remote.GetGeneration() {
			(&controlPlaneList.Items[controlPlaneIndex]).Spec.DeepCopyInto(&remote.Spec)
			remote.Annotations[v1alpha1.LastSyncGenerationRuntime] = strconv.Itoa(int(remote.GetGeneration() + 1))
			diffToApply = append(diffToApply, remote)
		}
	}
	return diffToApply, diffToDelete
}

func prepareControlPlaneTemplateForRuntime(controlPlane *v1alpha1.ModuleTemplate) {
	// we reset resource version and uid as we want to create new objects from control Plane diffs
	controlPlane.SetResourceVersion("")
	controlPlane.SetUID("")

	controlPlane.ObjectMeta.SetManagedFields([]metav1.ManagedFieldsEntry{})

	if controlPlane.Annotations == nil {
		controlPlane.Annotations = make(map[string]string)
	}
	controlPlane.Annotations[v1alpha1.LastSyncGenerationControlPlane] = strconv.Itoa(int(controlPlane.GetGeneration()))
	controlPlane.Annotations[v1alpha1.LastSyncGenerationRuntime] = strconv.Itoa(1)
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
) error {
	var moduleTemplatesRuntime *v1alpha1.ModuleTemplateList
	err := c.runtimeClient.List(ctx, moduleTemplatesRuntime)
	if err != nil {
		return err
	}
	for i := range moduleTemplatesRuntime.Items {
		if err := c.runtimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
			!k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *RemoteCatalog) CreateModuleTemplateCRDInRuntime(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = c.controlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = c.runtimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) {
		return c.runtimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
		})
	}

	if err != nil {
		return err
	}

	return nil
}
