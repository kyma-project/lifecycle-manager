package remote

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

var ErrTemplateCRDNotReady = errors.New("module template crd for catalog sync is not ready")

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
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1beta1.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

func NewRemoteCatalogFromKyma(kyma *v1beta1.Kyma) *RemoteCatalog {
	force := true
	return NewRemoteCatalog(
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: "catalog-sync", Force: &force},
			Namespace:       kyma.Spec.Sync.Namespace,
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
	kcp *v1beta1.ModuleTemplateList,
	kyma *v1beta1.Kyma,
) error {
	syncContext := SyncContextFromContext(ctx)

	if err := c.createOrUpdateCatalog(ctx, kcp, syncContext, kyma); err != nil {
		return err
	}

	moduleTemplatesRuntime := &v1beta1.ModuleTemplateList{}
	if err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime); err != nil {
		// it can happen that the ModuleTemplate CRD is not caught during to apply if there are no modules to apply
		// if this is the case and there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}

	return c.deleteDiffCatalog(ctx, kcp, moduleTemplatesRuntime, syncContext)
}

func (c *RemoteCatalog) deleteDiffCatalog(ctx context.Context,
	kcp *v1beta1.ModuleTemplateList,
	moduleTemplatesRuntime *v1beta1.ModuleTemplateList,
	syncContext *KymaSynchronizationContext,
) error {
	diffsToDelete := c.diffsToDelete(moduleTemplatesRuntime, kcp)
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
	kcp *v1beta1.ModuleTemplateList,
	syncContext *KymaSynchronizationContext,
	kyma *v1beta1.Kyma,
) error {
	channelLength := len(kcp.Items)
	results := make(chan error, channelLength)
	for kcpIndex := range kcp.Items {
		kcpIndex := kcpIndex
		go func() {
			c.prepareForSSA(&kcp.Items[kcpIndex])
			results <- c.patchDiff(ctx, &kcp.Items[kcpIndex], syncContext, false)
		}()
	}
	var errs []error
	for i := 0; i < channelLength; i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster when we apply it and retry
	if containsMetaIsNoMatchErr(errs) || len(errs) == 0 {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1beta1.ModuleTemplateKind.Plural(), kyma); err != nil {
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
	ctx context.Context, diff *v1beta1.ModuleTemplate, syncContext *KymaSynchronizationContext,
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

// diffsToDelete takes 2 v1beta1.ModuleTemplateList to then calculate any diffs.
// Diffs are defined as any v1beta1.ModuleTemplate that is available in the skrList but not in the kcpList.
func (c *RemoteCatalog) diffsToDelete(
	skrList *v1beta1.ModuleTemplateList, kcpList *v1beta1.ModuleTemplateList,
) []*v1beta1.ModuleTemplate {
	kcp := kcpList.Items
	skr := skrList.Items
	toDelete := make([]*v1beta1.ModuleTemplate, 0, len(skrList.Items))
	presentInKCP := make(map[string]struct{}, len(kcp))
	for i := range kcp {
		presentInKCP[kcp[i].Namespace+kcp[i].Name] = struct{}{}
	}
	for i := range skr {
		if _, inKCP := presentInKCP[skr[i].Namespace+skr[i].Name]; !inKCP {
			toDelete = append(toDelete, &skr[i])
		}
	}
	return toDelete
}

func (c *RemoteCatalog) prepareForSSA(moduleTemplate *v1beta1.ModuleTemplate) {
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
	moduleTemplatesRuntime := &v1beta1.ModuleTemplateList{Items: []v1beta1.ModuleTemplate{}}
	if err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime); err != nil {
		// if there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	for i := range moduleTemplatesRuntime.Items {
		if err := syncContext.RuntimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
			!k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *RemoteCatalog) CreateModuleTemplateCRDInRuntime(ctx context.Context, plural string, kyma *v1beta1.Kyma) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}

	syncContext := SyncContextFromContext(ctx)

	var err error
	err = syncContext.ControlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1beta1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = syncContext.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1beta1.GroupVersion.Group),
	}, crdFromRuntime)

	if err != nil {
		return err
	}

	latestGeneration := strconv.FormatInt(crd.Generation, 10)
	if k8serrors.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta1.GroupVersion.Version) ||
		!ContainsLatestCRDGeneration(kyma.Annotations[v1beta1.KcpModuleTemplateCRDGenerationAnnotation], latestGeneration) ||
		!ContainsLatestCRDGeneration(kyma.Annotations[v1beta1.SkrModuleTemplateCRDGenerationAnnotation], strconv.FormatInt(crdFromRuntime.Generation, 10)) {
		err = PatchCRD(ctx, syncContext.RuntimeClient, crd)
		if err == nil {
			if kyma.Annotations == nil {
				kyma.Annotations = make(map[string]string)
			}
			err = syncContext.RuntimeClient.Get(ctx, client.ObjectKey{
				Name: fmt.Sprintf("%s.%s", plural, v1beta1.GroupVersion.Group),
			}, crdFromRuntime)
			if err == nil {
				kyma.Annotations[v1beta1.KcpModuleTemplateCRDGenerationAnnotation] = latestGeneration
				kyma.Annotations[v1beta1.SkrModuleTemplateCRDGenerationAnnotation] = strconv.FormatInt(crdFromRuntime.Generation, 10)
				if err = syncContext.ControlPlaneClient.Update(ctx, kyma); err != nil {
					return err
				}
			}
		}
		return err
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
