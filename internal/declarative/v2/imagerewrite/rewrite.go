package imagerewrite

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	minSegmentsInImageReference = 2 // At least <host>/<image>:<tag>
)

var (
	ErrInvalidImageReference               = errors.New("invalid docker image reference")
	ErrMissingSlashInImageReference        = fmt.Errorf("%w: missing '/'", ErrInvalidImageReference)
	ErrFindingImageInPodContainer          = errors.New("error finding image in pod container")
	ErrFindingEnvVarsInPodContainer        = errors.New("error finding env vars in pod container")
	ErrUnexpectedEnvVarType                = errors.New("unexpected environment variable type")
	ErrFailedToSetNewImageInPodContainer   = errors.New("failed to set new image in pod container")
	ErrFailedToSetNewEnvListInPodContainer = errors.New("failed to set new env list in pod container")
)

// NameAndTag represents the docker image name and tag in the format <image>:<tag>.
type NameAndTag = string

// TargetImage represents a docker image reference that should be used instead of an original one embedded in a K8s manifest.
//
//	The overall format of a reference is <host[:port][/path]>/<image>:<tag>
type TargetImage struct {
	HostAndPath string
	NameAndTag
}

func (t *TargetImage) From(imageRef string) error {
	// split on last forward slash to separate host and path from image and tag
	lastSep := strings.LastIndex(imageRef, "/")
	if lastSep == -1 {
		return fmt.Errorf("parsing %q: %w", imageRef, ErrMissingSlashInImageReference)
	}
	t.HostAndPath = imageRef[:lastSep]
	t.NameAndTag = imageRef[lastSep+1:]

	return nil
}

func (t TargetImage) String() string {
	return fmt.Sprintf("%s/%s", t.HostAndPath, t.NameAndTag)
}

func (t TargetImage) Matches(imageRef string) bool {
	return strings.HasSuffix(imageRef, t.NameAndTag)
}

type PodContainerImageRewriter struct{}

func (r *PodContainerImageRewriter) Rewrite(targetImages []TargetImage, podContainer *unstructured.Unstructured) error {
	existingImageValue, found, err := unstructured.NestedString(podContainer.Object, "image")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrFindingImageInPodContainer, err.Error())
	}
	if !found {
		return fmt.Errorf("%w: not found", ErrFindingImageInPodContainer)
	}

	for _, replacement := range targetImages {
		// We know that existingImageValue is a docker image reference, so we only have the verify if the <name>:<tag> matches.
		if replacement.Matches(existingImageValue) {
			if err := unstructured.SetNestedField(podContainer.Object, replacement.String(), "image"); err != nil {
				return fmt.Errorf("%w: %v", ErrFailedToSetNewImageInPodContainer, err.Error())
			}
			break
		}
	}

	return nil
}

// PodContainerEnvsRewriter is a rewriter that rewrites container env vars in a Kubernetes manifest.
type PodContainerEnvsRewriter struct{}

func (r *PodContainerEnvsRewriter) Rewrite(targetImages []TargetImage, podContainer *unstructured.Unstructured) error {
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
		existingEnvValueStr, ok := existingEnvValue.(string)
		if !ok {
			return fmt.Errorf("%w: invalid type for value: %T (expected a string)", ErrUnexpectedEnvVarType, existingEnvValue)
		}
		if !found {
			continue // No value to rewrite
		}
		for _, replacement := range targetImages {
			// Check if the existing environment variable value is an image reference suitable for the replacement.
			if isSuitableForReplacement(existingEnvValueStr, replacement.NameAndTag) {
				envVar["value"] = replacement.String() // Set the new image reference
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

// isSuitableForReplacement checks if the given imageReference:
//   - is a docker image reference (simple heuristics)
//   - matches the provided nameAndTag
func isSuitableForReplacement(imageReference string, nameAndTag NameAndTag) bool {
	if !strings.HasSuffix(imageReference, nameAndTag) {
		return false // If it doesn't have the expected suffix, it certainly doesn't match
	}
	// We know it has the suffix, let's verify some invariants relevant for image references
	parts := strings.Split(imageReference, "/")
	if len(parts) < minSegmentsInImageReference {
		return false // Not enough parts to be a valid image reference: <host+port>/<path>/<image>:<tag>
	}
	// Check if the first part is a DNS name with at least one dot or refers to "localhost"
	if !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], "localhost") {
		return false
	}

	return true
}

func AsTargetImages(localizedImages []string) ([]TargetImage, error) {
	targetImages := make([]TargetImage, len(localizedImages))
	for i, img := range localizedImages {
		if err := targetImages[i].From(img); err != nil {
			return nil, err
		}
	}
	return targetImages, nil
}
