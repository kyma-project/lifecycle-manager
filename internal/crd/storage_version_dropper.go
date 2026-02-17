package crd

import (
	"context"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

const resourceVersionPairCount = 2

func DropStoredVersion(ctx context.Context, kcpClient client.Client, versionsToBeDropped string) {
	logger := ctrl.Log.WithName("storage-version-migration")
	versionsToBeDroppedMap := ParseStorageVersionsMap(versionsToBeDropped)
	logger.V(log.DebugLevel).Info(fmt.Sprintf("Handling dropping stored versions for, %v",
		versionsToBeDroppedMap))
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := kcpClient.List(ctx, crdList); err != nil {
		logger.V(log.InfoLevel).Error(err, "unable to list CRDs")
	}

	for _, crdItem := range crdList.Items {
		storedVersionToDrop, crdFound := versionsToBeDroppedMap[crdItem.Spec.Names.Kind]
		if crdItem.Spec.Group != shared.OperatorGroup || !crdFound {
			continue
		}
		logger.V(log.InfoLevel).Info(fmt.Sprintf("Checking the storedVersions for %s crd", crdItem.Spec.Names.Kind))
		oldStoredVersions := crdItem.Status.StoredVersions
		newStoredVersions := make([]string, 0, len(oldStoredVersions))
		for _, stored := range oldStoredVersions {
			if stored != storedVersionToDrop {
				newStoredVersions = append(newStoredVersions, stored)
			}
		}
		crdItem.Status.StoredVersions = newStoredVersions
		logger.V(log.InfoLevel).Info(fmt.Sprintf("The new storedVersions are %v", newStoredVersions))
		crd := crdItem
		if err := kcpClient.Status().Update(ctx, &crd); err != nil {
			msg := fmt.Sprintf("Failed to update CRD to remove %s from stored versions", storedVersionToDrop)
			logger.V(log.InfoLevel).Error(err, msg)
		}
	}
}

func ParseStorageVersionsMap(versions string) map[string]string {
	versionsToBeDroppedMap := map[string]string{}
	for pair := range strings.SplitSeq(versions, ",") {
		if kv := strings.Split(pair, ":"); len(kv) == resourceVersionPairCount {
			versionsToBeDroppedMap[kv[0]] = kv[1]
		}
	}

	return versionsToBeDroppedMap
}
