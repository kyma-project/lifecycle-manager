package img

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LayerName string

const (
	OCIRepresentationType = "oci-ref"
)

const (
	ConfigLayer LayerName = "config"
	CRDsLayer   LayerName = "crds"
)

type LayerRepresentation interface {
	ToInstallRaw() ([]byte, error)
}

type OCI struct {
	Repo               string                `json:"repo"`
	Name               string                `json:"name"`
	Ref                string                `json:"ref"`
	Type               string                `json:"type"`
	CredSecretSelector *metav1.LabelSelector `json:"credSecretSelector,omitempty"`
}

func (o *OCI) ToInstallRaw() ([]byte, error) {
	return json.Marshal(o)
}

func (o *OCI) String() string {
	return fmt.Sprintf("%s/%s:%s", o.Repo, o.Name, o.Ref)
}

type (
	LayerType string
	Layer     struct {
		LayerName
		LayerRepresentation
	}
)

type Layers []Layer
