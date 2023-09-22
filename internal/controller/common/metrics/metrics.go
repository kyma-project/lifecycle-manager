package metrics

import (
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"strings"
)

var (
	errMissingShootAnnotation = fmt.Errorf("expected annotation '%s' not found", v1beta2.SKRDomainAnnotation)
	errShootAnnotationNoValue = fmt.Errorf("annotation '%s' has empty value", v1beta2.SKRDomainAnnotation)
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

var (
	errMissingInstanceLabel = fmt.Errorf("expected label '%s' not found", v1beta2.InstanceIDLabel)
	errInstanceLabelNoValue = fmt.Errorf("label '%s' has empty value", v1beta2.InstanceIDLabel)
)

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
