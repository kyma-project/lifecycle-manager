package v1beta1

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/xeipuuv/gojsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/pkg/types"
)

// ImageSpec defines OCI Image specifications.
// +k8s:deepcopy-gen=true
type ImageSpec struct {
	// Repo defines the Image repo
	Repo string `json:"repo,omitempty"`

	// Name defines the Image name
	Name string `json:"name,omitempty"`

	// Ref is either a sha value, tag or version
	Ref string `json:"ref,omitempty"`

	// Type specifies the type of installation specification
	// that could be provided as part of a custom resource.
	// This time is used in codec to successfully decode from raw extensions.
	// +kubebuilder:validation:Enum=helm-chart;oci-ref;"kustomize";""
	Type RefTypeMetadata `json:"type,omitempty"`

	// CredSecretSelector is an optional field, for OCI image saved in private registry,
	// use it to indicate the secret which contains registry credentials,
	// must exist in the namespace same as manifest
	CredSecretSelector *metav1.LabelSelector `json:"credSecretSelector,omitempty"`
}

// HelmChartSpec defines the specification for a helm chart.
// +k8s:deepcopy-gen=true
type HelmChartSpec struct {
	// URL defines the helm repo URL
	// +kubebuilder:validation:Optional
	URL string `json:"url"`

	// ChartName defines the helm chart name
	// +kubebuilder:validation:Optional
	ChartName string `json:"chartName"`

	// Type defines the chart as "helm-chart"
	// +kubebuilder:validation:Optional
	Type RefTypeMetadata `json:"type"`
}

// KustomizeSpec defines the specification for a Kustomize specification.
// +k8s:deepcopy-gen=true
type KustomizeSpec struct {
	// Path defines the Kustomize local path
	Path string `json:"path"`

	// URL defines the Kustomize remote URL
	URL string `json:"url"`

	// Type defines the chart as "kustomize"
	// +kubebuilder:validation:Optional
	Type RefTypeMetadata `json:"type"`
}

type RefTypeMetadata string

func (t RefTypeMetadata) NotEmpty() bool {
	return t != NilRefType
}

const (
	HelmChartType RefTypeMetadata = "helm-chart"
	OciRefType    RefTypeMetadata = "oci-ref"
	KustomizeType RefTypeMetadata = "kustomize"
	NilRefType    RefTypeMetadata = ""
)

func GetSpecType(data []byte) (RefTypeMetadata, error) {
	raw := make(map[string]json.RawMessage)
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", err
	}

	var refType RefTypeMetadata
	if err := yaml.Unmarshal(raw["type"], &refType); err != nil {
		return "", err
	}

	return refType, nil
}

// +kubebuilder:object:generate=false
type Codec struct {
	imageSpecSchema     *gojsonschema.Schema
	helmChartSpecSchema *gojsonschema.Schema
	kustomizeSpecSchema *gojsonschema.Schema
}

func NewCodec() (*Codec, error) {
	imageSpecJSONBytes := jsonschema.Reflect(ImageSpec{})
	bytes, err := imageSpecJSONBytes.MarshalJSON()
	if err != nil {
		return nil, err
	}

	imageSpecSchema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(bytes))
	if err != nil {
		return nil, err
	}

	helmChartSpecJSONBytes := jsonschema.Reflect(HelmChartSpec{})
	bytes, err = helmChartSpecJSONBytes.MarshalJSON()
	if err != nil {
		return nil, err
	}

	helmChartSpecSchema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(bytes))
	if err != nil {
		return nil, err
	}

	kustomizeSpecJSONBytes := jsonschema.Reflect(KustomizeSpec{})
	bytes, err = kustomizeSpecJSONBytes.MarshalJSON()
	if err != nil {
		return nil, err
	}

	kustomizeSpecSchema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(bytes))
	if err != nil {
		return nil, err
	}

	return &Codec{
		imageSpecSchema:     imageSpecSchema,
		helmChartSpecSchema: helmChartSpecSchema,
		kustomizeSpecSchema: kustomizeSpecSchema,
	}, nil
}

func (c *Codec) Decode(data []byte, obj interface{}, refType RefTypeMetadata) error {
	if err := c.Validate(data, refType); err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &obj); err != nil {
		return err
	}

	return nil
}

var ErrInstallationTypeNotSupported = errors.New("installation type is not supported")

func (c *Codec) Validate(data []byte, refType RefTypeMetadata) error {
	dataBytes := gojsonschema.NewBytesLoader(data)
	var result *gojsonschema.Result
	var err error

	switch refType {
	case HelmChartType:
		result, err = c.helmChartSpecSchema.Validate(dataBytes)
		if err != nil {
			return err
		}
	case OciRefType:
		result, err = c.imageSpecSchema.Validate(dataBytes)
		if err != nil {
			return err
		}
	case KustomizeType:
		result, err = c.kustomizeSpecSchema.Validate(dataBytes)
		if err != nil {
			return err
		}
	case NilRefType:
		return fmt.Errorf("%s is invalid: %w", refType, ErrInstallationTypeNotSupported)
	}

	if !result.Valid() {
		errs := make([]error, 0, len(result.Errors()))
		for _, err := range result.Errors() {
			errs = append(errs, errors.New(err.String())) //nolint:goerr113
		}
		return types.NewMultiError(errs)
	}
	return nil
}
