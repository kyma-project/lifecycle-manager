package img

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var ErrLayerParsing = errors.New("layer could not be parsed")

type LayerRepresentation interface {
	ToInstallRaw() ([]byte, error)
}

type OCI struct {
	Repo string `json:"repo"`
	Name string `json:"name"`
	Ref  string `json:"ref"`
	Type string `json:"type"`
}

func (o *OCI) ToInstallRaw() ([]byte, error) {
	bytes, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to raw bytes: %w", err)
	}
	return bytes, nil
}

func (o *OCI) String() string {
	return fmt.Sprintf("%s/%s:%s", o.Repo, o.Name, o.Ref)
}

type (
	LayerType string
	Layer     struct {
		v1beta2.LayerName
		LayerRepresentation
	}
)

type Layers []Layer

func (l Layer) ConvertToImageSpec(ociRepo string) (*v1beta2.ImageSpec, error) {
	ociImage, ok := l.LayerRepresentation.(*OCI)
	if !ok {
		return nil, fmt.Errorf("%w: not an OCIImage", ErrLayerParsing)
	}

	return &v1beta2.ImageSpec{
		Repo: ociRepo, // Note: we override the repo here with an explicit value
		Name: ociImage.Name,
		Ref:  ociImage.Ref,
		Type: v1beta2.RefTypeMetadata(ociImage.Type),
	}, nil
}
