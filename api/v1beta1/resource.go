package v1beta1

import (
	"strings"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Resource identifies a Kubernetes object by GroupVersionKind, name and namespace.
// +k8s:deepcopy-gen=true
type Resource struct {
	apimetav1.GroupVersionKind `json:",inline"`

	Name      string `json:"name"`
	Namespace string `json:"namespace"`
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
