package manifest

import (
	"context"
	"errors"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

func WithClientCacheKey() declarativev2.WithClientCacheKeyOption {
	cacheKey := func(ctx context.Context, resource declarativev2.Object) (string, bool) {
		logger := logf.FromContext(ctx)
		manifest, ok := resource.(*v1beta2.Manifest)
		if !ok {
			return "", false
		}

		labelValue, err := internal.GetResourceLabel(resource, shared.KymaName)
		var labelErr *types.LabelNotFoundError
		if errors.As(err, &labelErr) {
			objectKey := client.ObjectKeyFromObject(resource)
			logger.V(internal.DebugLogLevel).Info(
				"client can not been cached due to lack of expected label",
				"resource", objectKey)
			return "", false
		}
		cacheKey := GenerateCacheKey(labelValue, manifest.GetNamespace())
		return cacheKey, true
	}
	return declarativev2.WithClientCacheKeyOption{ClientCacheKeyFn: cacheKey}
}

func GenerateCacheKey(values ...string) string {
	return strings.Join(values, "|")
}
