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
	errMissingShootAnnotation = fmt.Errorf("expected annotation '%s' not found", v1beta2.SKRDomainAnnotation)
	errShootAnnotationNoValue = fmt.Errorf("annotation '%s' has empty value", v1beta2.SKRDomainAnnotation)
	errMissingInstanceLabel   = fmt.Errorf("expected label '%s' not found", v1beta2.InstanceIDLabel)
	errInstanceLabelNoValue   = fmt.Errorf("label '%s' has empty value", v1beta2.InstanceIDLabel)
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
		return "", errMissingShootAnnotation
	}
	if shoot == "" {
		return shoot, errShootAnnotationNoValue
	}
	return shoot, nil
}

func ExtractInstanceID(kyma *v1beta2.Kyma) (string, error) {
	instanceID, keyExists := kyma.Labels[v1beta2.InstanceIDLabel]
	if !keyExists {
		return "", errMissingInstanceLabel
	}
	if instanceID == "" {
		return instanceID, errInstanceLabelNoValue
	}
	return instanceID, nil
}

func IsMissingMetricsAnnotationOrLabel(err error) bool {
	return errors.Is(err, errInstanceLabelNoValue) ||
		errors.Is(err, errMissingInstanceLabel) ||
		errors.Is(err, errShootAnnotationNoValue) ||
		errors.Is(err, errMissingShootAnnotation)
}
