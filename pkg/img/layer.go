package img

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LayerName string

const (
	ConfigLayer LayerName = "config"
)

type OCI struct {
	Repo               string                `json:"repo"`
	Name               string                `json:"name"`
	Ref                string                `json:"ref"`
	Type               string                `json:"type"`
	CredSecretSelector *metav1.LabelSelector `json:"credSecretSelector,omitempty"`
}

func (o *OCI) String() string {
	return fmt.Sprintf("%s/%s:%s", o.Repo, o.Name, o.Ref)
}

type (
	LayerType string
	Layer     struct {
		LayerName
		LayerRepresentation *OCI
	}
)

type Layers []Layer
