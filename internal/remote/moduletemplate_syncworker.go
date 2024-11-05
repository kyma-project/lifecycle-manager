package remote

import (
	"context"
	"errors"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	errModuleTemplateCRDNotReady = errors.New("catalog sync: ModuleTemplate CRD is not ready")
	errModuleTemplateCleanup     = errors.New("catalog sync: Failed to delete obsolete ModuleTemplates")
	errCatModuleTemplatesApply   = errors.New("catalog sync: Could not apply ModuleTemplates")
)

// moduleTemplateConcurrentWorker performs ModuleTemplate synchronization using multiple goroutines.
type moduleTemplateConcurrentWorker struct {
	namespace  string
	patchDiff  func(ctx context.Context, obj *v1beta2.ModuleTemplate) error
	deleteDiff func(ctx context.Context, obj *v1beta2.ModuleTemplate) error
	createCRD  func(ctx context.Context) error
}

// newModuleTemplateConcurrentWorker returns a new moduleTemplateConcurrentWorker instance with default dependencies.
func newModuleTemplateConcurrentWorker(kcpClient, skrClient client.Client, settings *Settings) *moduleTemplateConcurrentWorker {
	patchDiffFn := func(ctx context.Context, obj *v1beta2.ModuleTemplate) error {
		return patchDiffModuleTemplate(ctx, obj, skrClient, settings.SSAPatchOptions)
	}

	deleteDiffFn := func(ctx context.Context, obj *v1beta2.ModuleTemplate) error {
		return deleteModuleTemplate(ctx, obj, skrClient)
	}

	createCRDFn := func(ctx context.Context) error {
		return createModuleTemplateCRDInRuntime(ctx, kcpClient, skrClient)
	}

	return &moduleTemplateConcurrentWorker{
		namespace:  settings.Namespace,
		patchDiff:  patchDiffFn,
		deleteDiff: deleteDiffFn,
		createCRD:  createCRDFn,
	}
}

// SyncConcurrently synchronizes ModuleTemplates from KCP to SKR.
// kcpModules are the ModuleTemplates to be synced from the KCP cluster.
func (c *moduleTemplateConcurrentWorker) SyncConcurrently(ctx context.Context, kcpModules []v1beta2.ModuleTemplate) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		go func() {
			prepareModuleTemplateForSSA(&kcpModules[kcpIndex], c.namespace)
			results <- c.patchDiff(ctx, &kcpModules[kcpIndex])
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// retry if ModuleTemplate CRD is not existing in SKR cluster
	if containsCRDNotFoundError(errs) {
		if err := c.createCRD(ctx); err != nil {
			return err
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errCatModuleTemplatesApply)
		return errors.Join(errs...)
	}
	return nil
}

// DeleteConcurrently deletes ModuleTemplates from SKR.
func (c *moduleTemplateConcurrentWorker) DeleteConcurrently(ctx context.Context,
	diffsToDelete []v1beta2.ModuleTemplate,
) error {
	channelLength := len(diffsToDelete)
	results := make(chan error, channelLength)
	for _, diff := range diffsToDelete {
		go func() {
			results <- c.deleteDiff(ctx, &diff)
		}()
	}
	var errs []error
	for range channelLength {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errModuleTemplateCleanup)
		return errors.Join(errs...)
	}
	return nil
}

func createModuleTemplateCRDInRuntime(ctx context.Context, kcpClient client.Client, skrClient client.Client) error {
	kcpCrd := &apiextensionsv1.CustomResourceDefinition{}
	skrCrd := &apiextensionsv1.CustomResourceDefinition{}
	objKey := client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", shared.ModuleTemplateKind.Plural(), v1beta2.GroupVersion.Group),
	}
	err := kcpClient.Get(ctx, objKey, kcpCrd)
	if err != nil {
		return fmt.Errorf("failed to get ModuleTemplate CRD from KCP: %w", err)
	}

	err = skrClient.Get(ctx, objKey, skrCrd)

	if util.IsNotFound(err) || !ContainsLatestVersion(skrCrd, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, skrClient, kcpCrd)
	}

	if !crdReady(skrCrd) {
		return errModuleTemplateCRDNotReady
	}

	if err != nil {
		return fmt.Errorf("failed to get ModuleTemplate CRD from SKR: %w", err)
	}

	return nil
}

func prepareModuleTemplateForSSA(moduleTemplate *v1beta2.ModuleTemplate, namespace string) {
	moduleTemplate.SetResourceVersion("")
	moduleTemplate.SetUID("")
	moduleTemplate.SetManagedFields([]apimetav1.ManagedFieldsEntry{})
	moduleTemplate.SetLabels(collections.MergeMaps(moduleTemplate.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	if namespace != "" {
		moduleTemplate.SetNamespace(namespace)
	}
}

func patchDiffModuleTemplate(ctx context.Context, diff *v1beta2.ModuleTemplate, skrClient client.Client, ssaPatchOptions *client.PatchOptions) error {
	err := skrClient.Patch(
		ctx, diff, client.Apply, ssaPatchOptions,
	)
	if err != nil {
		return fmt.Errorf("could not apply ModuleTemplate diff: %w", err)
	}
	return nil
}

func deleteModuleTemplate(
	ctx context.Context, diff *v1beta2.ModuleTemplate, skrClient client.Client,
) error {
	err := skrClient.Delete(ctx, diff)
	if err != nil {
		return fmt.Errorf("could not delete ModuleTemplate: %w", err)
	}
	return nil
}
