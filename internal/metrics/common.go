package metrics

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	shootIDLabel    = "shoot"
	instanceIDLabel = "instance_id"
	kymaNameLabel   = "kyma_name"
)

var (
	ErrMissingShootAnnotation = fmt.Errorf("expected annotation '%s' not found", v1beta2.SKRDomainAnnotation)
	ErrShootAnnotationNoValue = fmt.Errorf("annotation '%s' has empty value", v1beta2.SKRDomainAnnotation)
	errMetric                 = errors.New("failed to update metrics")
)

func ExtractShootID(kyma *v1beta2.Kyma) (string, error) {
	shoot := ""
	shootFQDN, keyExists := kyma.Annotations[v1beta2.SKRDomainAnnotation]
	if keyExists {
		parts := strings.Split(shootFQDN, ".")
		minFqdnParts := 2
		if len(parts) > minFqdnParts {
			shoot = parts[0] // hostname
		}
	}
	if !keyExists {
		return "", ErrMissingShootAnnotation
	}
	if shoot == "" {
		return shoot, ErrShootAnnotationNoValue
	}
	return shoot, nil
}

var (
	ErrMissingInstanceLabel = fmt.Errorf("expected label '%s' not found", v1beta2.InstanceIDLabel)
	ErrInstanceLabelNoValue = fmt.Errorf("label '%s' has empty value", v1beta2.InstanceIDLabel)
)

func ExtractInstanceID(kyma *v1beta2.Kyma) (string, error) {
	instanceID, keyExists := kyma.Labels[v1beta2.InstanceIDLabel]
	if !keyExists {
		return "", ErrMissingInstanceLabel
	}
	if instanceID == "" {
		return instanceID, ErrInstanceLabelNoValue
	}
	return instanceID, nil
}
