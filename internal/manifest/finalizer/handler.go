package finalizer

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/labelsremoval"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
)

const DefaultFinalizer = "declarative.kyma-project.io/finalizer"

func RemoveFinalizers(manifest *v1beta2.Manifest, originalErr error) bool {
	finalizersToRemove := []string{DefaultFinalizer, labelsremoval.LabelRemovalFinalizer}
	if errors.Is(originalErr, common.ErrAccessSecretNotFound) || manifest.IsUnmanaged() {
		finalizersToRemove = manifest.GetFinalizers()
	}
	finalizerRemoved := false
	for _, f := range finalizersToRemove {
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
