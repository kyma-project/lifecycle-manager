package internal

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/types"
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
	countMap := map[string]bool{}
	for _, item := range items {
		unstructuredItem, ok := item.Object.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		id := getID(unstructuredItem)
		if countMap[id] {
			continue
		}
		countMap[id] = true
		objects.Items = append(objects.Items, unstructuredItem)
	}
	return *objects, nil
}

func getID(item *unstructured.Unstructured) string {
	return strings.Join([]string{
		item.GetNamespace(), item.GetName(),
		item.GroupVersionKind().Group, item.GroupVersionKind().Version, item.GroupVersionKind().Kind,
	}, "/")
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

func GetCacheOptions(labelSelector labels.Set) cache.Options {
	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&v1.Secret{}: {
				Label: labels.SelectorFromSet(
					labelSelector,
				),
			},
		},
	}
}
