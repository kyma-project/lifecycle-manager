package remote

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

var ErrTemplateCRDNotReady = errors.New("module template crd for catalog sync is not ready")

const moduleCatalogSyncFieldManager = "catalog-sync"

type Settings struct {
	// this namespace flag can be used to override the namespace in which all ModuleTemplates should be applied.
	Namespace       string
	SSAPatchOptions *client.PatchOptions
}

//nolint:revive
type RemoteCatalog struct {
	settings Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1beta2.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

func NewRemoteCatalogFromKyma(kyma *v1beta2.Kyma) *RemoteCatalog {
	force := true
	return NewRemoteCatalog(
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: moduleCatalogSyncFieldManager, Force: &force},
			Namespace:       kyma.Namespace,
		},
	)
}

// NewRemoteCatalog uses 2 Clients from a Sync Context to create a Catalog in a remote Cluster.
func NewRemoteCatalog(
	settings Settings,
) *RemoteCatalog {
	return &RemoteCatalog{settings: settings}
}

// CreateOrUpdate first lists all currently available moduleTemplates in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD but only if there are available crs to copy.
// It will use a 2 stage process:
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
func (c *RemoteCatalog) CreateOrUpdate(
	ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
) error {
	syncContext := SyncContextFromContext(ctx)

	if err := c.createOrUpdateCatalog(ctx, kcpModules, syncContext); err != nil {
		return err
	}

	runtimeModules := &v1beta2.ModuleTemplateList{}
	if err := syncContext.RuntimeClient.List(ctx, runtimeModules); err != nil {
		// it can happen that the ModuleTemplate CRD is not caught during to apply if there are no modules to apply
		// if this is the case and there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}

	return c.deleteDiffCatalog(ctx, kcpModules, runtimeModules.Items, syncContext)
}

func (c *RemoteCatalog) deleteDiffCatalog(ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
	runtimeModules []v1beta2.ModuleTemplate,
	syncContext *KymaSynchronizationContext,
) error {
	diffsToDelete := c.diffsToDelete(runtimeModules, kcpModules)
	channelLength := len(diffsToDelete)
	results := make(chan error, channelLength)
	for _, diff := range diffsToDelete {
		diff := diff
		go func() {
			results <- c.patchDiff(ctx, diff, syncContext, true)
		}()
	}
	var errs []error
	for i := 0; i < channelLength; i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("could not delete obsolete catalog templates: %w", types.NewMultiError(errs))
	}
	return nil
}

func (c *RemoteCatalog) createOrUpdateCatalog(ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
	syncContext *KymaSynchronizationContext,
) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		kcpIndex := kcpIndex
		go func() {
			c.prepareForSSA(&kcpModules[kcpIndex])
			results <- c.patchDiff(ctx, &kcpModules[kcpIndex], syncContext, false)
		}()
	}
	var errs []error
	for i := 0; i < channelLength; i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster when we apply it and retry
	if containsMetaIsNoMatchErr(errs) {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1beta2.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("could not apply catalog templates: %w", types.NewMultiError(errs))
	}
	return nil
}

func containsMetaIsNoMatchErr(errs []error) bool {
	for _, err := range errs {
		if meta.IsNoMatchError(errors.Unwrap(err)) {
			return true
		}
	}
	return false
}

func (c *RemoteCatalog) patchDiff(
	ctx context.Context, diff *v1beta2.ModuleTemplate, syncContext *KymaSynchronizationContext,
	deleteInsteadOfPatch bool,
) error {
	var err error
	if deleteInsteadOfPatch {
		err = syncContext.RuntimeClient.Delete(ctx, diff)
	} else {
		err = syncContext.RuntimeClient.Patch(
			ctx, diff, client.Apply, c.settings.SSAPatchOptions,
		)
	}

	if err != nil {
		return fmt.Errorf("could not apply module template diff: %w", err)
	}
	return nil
}

// diffsToDelete takes 2 v1beta2.ModuleTemplateList to then calculate any diffs.
// Diffs are defined as any v1beta2.ModuleTemplate that is available in the skrList but not in the kcpList.
func (c *RemoteCatalog) diffsToDelete(
	skrList []v1beta2.ModuleTemplate, kcpList []v1beta2.ModuleTemplate,
) []*v1beta2.ModuleTemplate {
	toDelete := make([]*v1beta2.ModuleTemplate, 0, len(skrList))
	presentInKCP := make(map[string]struct{}, len(kcpList))
	for i := range kcpList {
		presentInKCP[kcpList[i].Namespace+kcpList[i].Name] = struct{}{}
	}
	for i := range skrList {
		if _, inKCP := presentInKCP[skrList[i].Namespace+skrList[i].Name]; !inKCP  && isManagedByKcp(skrList[i]){
			toDelete = append(toDelete, &skrList[i])
		}
	}
	return toDelete
}

func isManagedByKcp(skrTemplate v1beta2.ModuleTemplate) bool {
	for _, managedFieldEntry := range skrTemplate.ObjectMeta.ManagedFields {
		if managedFieldEntry.Manager == moduleCatalogSyncFieldManager {
			return true
		}
	}
	return false
}

func (c *RemoteCatalog) prepareForSSA(moduleTemplate *v1beta2.ModuleTemplate) {
	moduleTemplate.SetResourceVersion("")
	moduleTemplate.SetUID("")
	moduleTemplate.SetManagedFields([]metav1.ManagedFieldsEntry{})

	if c.settings.Namespace != "" {
		moduleTemplate.SetNamespace(c.settings.Namespace)
	}
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
) error {
	syncContext := SyncContextFromContext(ctx)
	moduleTemplatesRuntime := &v1beta2.ModuleTemplateList{Items: []v1beta2.ModuleTemplate{}}
	if err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime); err != nil {
		// if there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	for i := range moduleTemplatesRuntime.Items {
		if isManagedByKcp(moduleTemplatesRuntime.Items[i]) {
			if err := syncContext.RuntimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
				!k8serrors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}

func (c *RemoteCatalog) CreateModuleTemplateCRDInRuntime(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}

	syncContext := SyncContextFromContext(ctx)

	var err error
	err = syncContext.ControlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = syncContext.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, syncContext.RuntimeClient, crd)
	}

	if !crdReady(crdFromRuntime) {
		return ErrTemplateCRDNotReady
	}

	if err != nil {
		return err
	}

	return nil
}

func crdReady(crd *v1extensions.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		if cond.Type == v1extensions.Established &&
			cond.Status == v1extensions.ConditionTrue {
			return true
		}

		if cond.Type == v1extensions.NamesAccepted &&
			cond.Status == v1extensions.ConditionFalse {
			// This indicates a naming conflict, but it's probably not the
			// job of this function to fail because of that. Instead,
			// we treat it as a success, since the process should be able to
			// continue.
			return true
		}
	}
	return false
}
