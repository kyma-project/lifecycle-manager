package internal

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DebugLogLevel = 2
	TraceLogLevel = 3
)

func ParseManifestToObjects(path string) (ManifestResources, error) {
	objects := &ManifestResources{}
	builder := resource.NewLocalBuilder().
		Unstructured().
		Path(false, path).
		Flatten().
		ContinueOnError()

	result := builder.Do()

	if err := result.Err(); err != nil {
		return ManifestResources{}, fmt.Errorf("parse manifest: %w", err)
	}
	items, err := result.Infos()
	if err != nil {
		return ManifestResources{}, fmt.Errorf("parse manifest to resource infos: %w", err)
	}

	for _, item := range items {
		unstructuredItem, ok := item.Object.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		objects.Items = append(objects.Items, unstructuredItem)
	}
	return *objects, nil
}

func GetResourceLabel(resource client.Object, labelName string) (string, error) {
	resourceLables := resource.GetLabels()
	labelValue, ok := resourceLables[labelName]
	if !ok {
		return "", &types.LabelNotFoundError{
			Resource:  resource,
			LabelName: labelValue,
		}
	}
	return labelValue, nil
}

func GetCacheFunc(labelSelector labels.Set) cache.NewCacheFunc {
	return cache.BuilderWithOptions(
		cache.Options{
			SelectorsByObject: cache.SelectorsByObject{
				&v1.Secret{}: {
					Label: labels.SelectorFromSet(
						labelSelector,
					),
				},
			},
		},
	)
}
