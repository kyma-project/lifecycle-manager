package img

import (
	"encoding/json"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LayerRepresentation interface {
	ToInstallRaw() ([]byte, error)
}

type OCI struct {
	Repo               string                   `json:"repo"`
	Name               string                   `json:"name"`
	Ref                string                   `json:"ref"`
	Type               string                   `json:"type"`
	CredSecretSelector *apimetav1.LabelSelector `json:"credSecretSelector,omitempty"`
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
