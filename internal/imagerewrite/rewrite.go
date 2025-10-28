package imagerewrite

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrInvalidImageReference               = errors.New("invalid docker image reference")
	ErrFindingImageInPodContainer          = errors.New("error finding image in pod container")
	ErrFindingEnvVarsInPodContainer        = errors.New("error finding env vars in pod container")
	ErrUnexpectedEnvVarType                = errors.New("unexpected environment variable type")
	ErrFailedToSetNewImageInPodContainer   = errors.New("failed to set new image in pod container")
	ErrFailedToSetNewEnvListInPodContainer = errors.New("failed to set new env list in pod container")
)

func IsSupportedKind(kind string) bool {
	return kind == "Deployment" || kind == "StatefulSet"
}

// NameAndTag represents the Docker image name and tag in the format <image>:<tag>.
type NameAndTag string

// DockerImageReference represents a Docker image reference used in the rewriting mechanism.
//
//	The overall format of a reference is: <host[:port][/path]>/<image>:<tag>[@<digest>]
type DockerImageReference struct {
	NameAndTag

	HostAndPath string
	Digest      string
}

// Regex matches prefix@sha256:checksum, prefix:version, and prefix:version@sha256:checksum.
var ociImagePattern = regexp.MustCompile(
	`^(?:(?P<host>[\w.-]+(?::\d+)?(?:/[\w.-]+)*)/)?` +
		`(?P<name>[a-z0-9]+(?:[_-][a-z0-9]+)*)` +
		`(?:` +
		`(?::(?P<tag>[\w.\-]+))(?:@(?P<digest>sha256:[a-fA-F0-9]{64}))?|` +
		`@(?P<digest>sha256:[a-fA-F0-9]{64})` +
		`)` +
		`$`,
)

func NewDockerImageReference(val string) (*DockerImageReference, error) {
	res := &DockerImageReference{}

	matches := ociImagePattern.FindStringSubmatch(val)
	if matches == nil {
		return nil, ErrInvalidImageReference
	}
	result := map[string]string{}
	for i, name := range ociImagePattern.SubexpNames() {
		if matches[i] == "" {
			// Skip empty captures because there are two "digest" groups and only one will match.
			continue
		}
		result[name] = matches[i]
	}

	res.HostAndPath = result["host"]
	imageName := result["name"]
	tag := result["tag"]

	if tag != "" {
		res.NameAndTag = NameAndTag(imageName + ":" + tag)
	} else {
		res.NameAndTag = NameAndTag(imageName)
	}

	if digest := result["digest"]; digest != "" {
		res.Digest = digest
	}

	return res, nil
}

func (ir *DockerImageReference) Matches(otherNameAndTag NameAndTag) bool {
	return ir.NameAndTag == otherNameAndTag
}

func (ir *DockerImageReference) String() string {
	if len(ir.Digest) > 0 {
		return fmt.Sprintf("%s/%s@%s", ir.HostAndPath, ir.NameAndTag, ir.Digest)
	}
	return fmt.Sprintf("%s/%s", ir.HostAndPath, ir.NameAndTag)
}

type PodContainerImageRewriter struct{}

func (r *PodContainerImageRewriter) Rewrite(
	targetImages []*DockerImageReference,
	podContainer *unstructured.Unstructured,
) error {
	existingImageValue, found, err := unstructured.NestedString(podContainer.Object, "image")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFindingImageInPodContainer, err.Error())
	}
	if !found {
		// No image found in the pod container, nothing to rewrite
		// Note: For some reasons the image attribute is marked as optional in the k8s sources:
		// k8s.io/api@v0.33.3/core/v1/types.go, line 2764
		return nil
	}

	existingImage, err := NewDockerImageReference(existingImageValue)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidImageReference, err.Error())
	}

	for _, targetImage := range targetImages {
		// We know that existingImage is a docker image reference,
		// so we only have the verify if the <name>:<tag> matches.
		if targetImage.Matches(existingImage.NameAndTag) {
			if err := unstructured.SetNestedField(podContainer.Object, targetImage.String(), "image"); err != nil {
				return fmt.Errorf("%w: %v", ErrFailedToSetNewImageInPodContainer, err.Error())
			}
			break
		}
	}

	return nil
}

// PodContainerEnvsRewriter is a rewriter that rewrites container env vars in a Kubernetes manifest.
type PodContainerEnvsRewriter struct{}

func (r *PodContainerEnvsRewriter) Rewrite(
	targetImages []*DockerImageReference,
	podContainer *unstructured.Unstructured,
) error {
	// Note: NestedSlice returns a COPY
	envEntries, found, err := unstructured.NestedSlice(podContainer.Object, "env")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFindingEnvVarsInPodContainer, err.Error())
	}
	if !found {
		return nil
	}

	if len(envEntries) == 0 {
		return nil // No environment variables to rewrite
	}

	for _, envEntry := range envEntries {
		envVar, ok := envEntry.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %T (expected map[string]any)", ErrUnexpectedEnvVarType, envEntry)
		}

		existingEnvValue, found := envVar["value"]
		if !found {
			continue // No `value` attribute (may use `valueFrom`), skip this environment variable
		}

		envVarValueStr, ok := existingEnvValue.(string)
		if !ok {
			return fmt.Errorf(
				"%w: invalid type for value: %T (expected a string)",
				ErrUnexpectedEnvVarType,
				existingEnvValue,
			)
		}
		for _, targetImage := range targetImages {
			// Check if the existing environment variable value is an image reference suitable for the replacement.
			if isImageRefForReplacement(envVarValueStr, targetImage.NameAndTag) {
				envVar["value"] = targetImage.String() // Set the new image reference
				break
			}
		}
	}

	// Set the modified environment variables back to the pod container
	if err = unstructured.SetNestedSlice(podContainer.Object, envEntries, "env"); err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToSetNewEnvListInPodContainer, err.Error())
	}
	return nil
}

// isImageRefForReplacement checks if the environment variable value is a Docker image reference
// suitable for replacement with the target image.
func isImageRefForReplacement(envVarValue string, targetNameAndTag NameAndTag) bool {
	if !strings.Contains(envVarValue, string(targetNameAndTag)) {
		return false // The envVarValue does not contain the targetRef.NameAndTag so it is not suitable for replacement.
	}

	// The envVarValue contains the targetRef.NameAndTag substring (e.g: "myimage:1.2.3"),
	// so it may be a Docker image referenence suitable for replacement.
	trySourceRef, err := NewDockerImageReference(envVarValue)
	if err != nil {
		return false // Not a valid Docker image reference, not suitable for replacement.
	}

	return trySourceRef.Matches(targetNameAndTag)
}

func AsImageReferences(vals []string) ([]*DockerImageReference, error) {
	res := make([]*DockerImageReference, 0, len(vals))
	for _, img := range vals {
		newRef, err := NewDockerImageReference(img)
		if err != nil {
			return nil, err
		}
		res = append(res, newRef)
	}
	return res, nil
}
