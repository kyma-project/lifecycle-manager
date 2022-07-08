package img

import "fmt"

type LayerName string

type LayerRepresentation interface {
	ToGenericRepresentation() map[string]any
}

type OCIRef struct {
	Repo   string
	Module string
	Digest string
}

func (r *OCIRef) ToGenericRepresentation() map[string]any {
	return map[string]any{
		"ociRef": map[string]any{
			"repo":   r.Repo,
			"module": r.Module,
			"ref":    r.Digest,
		},
	}
}

func (r *OCIRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.Repo, r.Module, r.Digest)
}

type HelmRef struct {
	ChartName string
	URL       string
	Version   string
}

func (r *HelmRef) ToGenericRepresentation() map[string]any {
	return map[string]any{
		"helmRef": map[string]any{
			"chartName": r.ChartName,
			"url":       r.URL,
			"version":   r.Version,
		},
	}
}

func (r *HelmRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.URL, r.ChartName, r.Version)
}

type (
	LayerType string
	Layer     struct {
		LayerName
		LayerRepresentation
		LayerType
	}
)

type Layers []Layer
