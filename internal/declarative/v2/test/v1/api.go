// +kubebuilder:object:generate=true
// +groupName=test.declarative.kyma-project.io
package v1

import (
	"fmt"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type TestAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TestAPISpec        `json:"spec,omitempty"`
	Status            declarative.Status `json:"status,omitempty"`
}

// TestAPISpec defines the desired state of TestAPI.
type TestAPISpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ManifestName string `json:"manifestName,omitempty"`
}

//+kubebuilder:object:root=true

// TestAPIList contains a list of TestAPI.
type TestAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestAPI `json:"items"`
}

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "test.declarative.kyma-project.io", Version: "v1"} //nolint:gochecknoglobals

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion} //nolint:gochecknoglobals

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme //nolint:gochecknoglobals
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&TestAPI{}, &TestAPIList{})
}

var _ declarative.Object = &TestAPI{}

func (s *TestAPI) GetStatus() declarative.Status {
	return s.Status
}

func (s *TestAPI) SetStatus(status declarative.Status) {
	s.Status = status
}

func (s *TestAPI) ComponentName() string {
	return fmt.Sprintf("test-api-%s-%s", s.Name, s.Spec.ManifestName)
}
