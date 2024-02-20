package cache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

type ObjectKey string

func GenerateObjectKey(name k8stypes.NamespacedName, groupVersionKind schema.GroupVersionKind) ObjectKey {
	return ObjectKey(fmt.Sprintf("%v:%v/%v/%v", name.String(), groupVersionKind.Group, groupVersionKind.Version, groupVersionKind.Kind))
}
