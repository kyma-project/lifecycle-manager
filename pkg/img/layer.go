package img

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LayerName string

const (
	OCIRepresentationType  = "oci-ref"
	HelmRepresentationType = "helm-chart"
)

const (
	ConfigLayer LayerName = "config"
	CRDsLayer   LayerName = "crds"
)

type LayerRepresentation interface{}

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

type Helm struct {
	ChartName string `json:"chartName"`
	URL       string `json:"url"`
	Version   string `json:"version"`
	Type      string `json:"type"`
}

func (h *Helm) String() string {
	return fmt.Sprintf("%s/%s:%s", h.URL, h.ChartName, h.Version)
}

type (
	LayerType string
	Layer     struct {
		LayerName
		LayerRepresentation
	}
)

type Layers []Layer
