package finalizer

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
)

const DefaultFinalizer = "declarative.kyma-project.io/finalizer"

// RemoveRequiredFinalizers removes preconfigured finalizers, but not include modulecr.CustomResourceManagerFinalizer.
func RemoveRequiredFinalizers(manifest *v1beta2.Manifest) bool {
	finalizersToRemove := []string{DefaultFinalizer, labelsremoval.LabelRemovalFinalizer}

	finalizerRemoved := false
	for _, f := range finalizersToRemove {
		if controllerutil.RemoveFinalizer(manifest, f) {
			finalizerRemoved = true
		}
	}
	return finalizerRemoved
}

func RemoveAllFinalizers(manifest *v1beta2.Manifest) bool {
	finalizerRemoved := false
	for _, f := range manifest.GetFinalizers() {
		if controllerutil.RemoveFinalizer(manifest, f) {
			finalizerRemoved = true
		}
	}
	return finalizerRemoved
}

func FinalizersUpdateRequired(manifest *v1beta2.Manifest) bool {
	defaultFinalizerAdded := controllerutil.AddFinalizer(manifest, DefaultFinalizer)
	labelRemovalFinalizerAdded := controllerutil.AddFinalizer(manifest, labelsremoval.LabelRemovalFinalizer)
	return defaultFinalizerAdded || labelRemovalFinalizerAdded
}
