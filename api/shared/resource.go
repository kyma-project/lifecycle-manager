package shared

import (
	"strings"

	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +k8s:deepcopy-gen=true
type Resource struct {
	Name                              string `json:"name"`
	Namespace                         string `json:"namespace"`
	apimachinerymeta.GroupVersionKind `json:",inline"`
}

func (r Resource) ToUnstructured() *unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind(r.GroupVersionKind))
	obj.SetName(r.Name)
	obj.SetNamespace(r.Namespace)
	return &obj
}

func (r Resource) ID() string {
	return strings.Join([]string{r.Namespace, r.Name, r.Group, r.Version, r.Kind}, "/")
}
