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

type LayerRepresentation interface {
	ToGenericRepresentation() map[string]any
}

type OCI struct {
	Repo               string
	Name               string
	Ref                string
	CredSecretSelector *metav1.LabelSelector
}

func (o *OCI) ToGenericRepresentation() map[string]any {
	return map[string]any{
		"repo":               o.Repo,
		"name":               o.Name,
		"ref":                o.Ref,
		"type":               OCIRepresentationType,
		"credSecretSelector": o.CredSecretSelector,
	}
}

func (o *OCI) String() string {
	return fmt.Sprintf("%s/%s:%s", o.Repo, o.Name, o.Ref)
}

type Helm struct {
	ChartName string
	URL       string
	Version   string
}

func (h *Helm) ToGenericRepresentation() map[string]any {
	return map[string]any{
		"chartName": h.ChartName,
		"url":       h.URL,
		"version":   h.Version,
		"type":      HelmRepresentationType,
	}
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
