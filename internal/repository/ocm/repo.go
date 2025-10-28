package ocm

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg/componentmapping"
)

var (
	ErrNoProtocolScheme = errors.New("host address must not contain protocol scheme (http/https)")
	ErrNoLeadingSlash   = errors.New("host address must not start with a '/'")
)

type OciRepositoryReader interface {
	Config(ctx context.Context, ref string) ([]byte, error)
	PullLayer(ctx context.Context, ref string) (containerregistryv1.Layer, error)
}

// RepositoryReader provides basic support to read OCM data from OCI repositories.
type RepositoryReader struct {
	hostWithPort        string
	ociRepositoryReader OciRepositoryReader
}

// NewRepository creates a new RepositoryReader for the given hostref.
// The host must not contain a protocol scheme (http/https), for example: "k3d-kcp-registry.localhost:5000".
func NewRepository(hostWithPort string,
	ociRepositoryReader OciRepositoryReader,
) (*RepositoryReader, error) {
	if strings.Contains(hostWithPort, "://") {
		return nil, fmt.Errorf("%w: %q", ErrNoProtocolScheme, hostWithPort)
	}

	if strings.HasPrefix(hostWithPort, "/") {
		return nil, fmt.Errorf("%w: %q", ErrNoLeadingSlash, hostWithPort)
	}

	reader := &RepositoryReader{
		hostWithPort,
		ociRepositoryReader,
	}

	return reader, nil
}

// GetConfigFile retrieves the config file as a byte slice for the OCM artifact.
// We're not using image-oriented types here because OCM artifacts "config file" is not a standard image config.
func (s *RepositoryReader) GetConfig(ctx context.Context, name, tag string) ([]byte, error) {
	ref := s.toImageRef(name, tag)
	configBytes, err := s.ociRepositoryReader.Config(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for ref=%q: %w", ref, err)
	}

	return configBytes, nil
}

// PullLayer retrieves a layer with given digest from the OCM artifact identified by name and tag.
func (s *RepositoryReader) PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error) {
	ref := s.toImageRef(name, tag)
	refWithDigest := fmt.Sprintf("%s@%s", ref, digest)
	configBytes, err := s.ociRepositoryReader.PullLayer(ctx, refWithDigest)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ref=%q: %w", refWithDigest, err)
	}
	return configBytes, nil
}

func (s *RepositoryReader) toImageRef(name, tag string) string {
	hostPath := path.Join(s.hostWithPort, componentmapping.ComponentDescriptorNamespace, name)
	return fmt.Sprintf("%s:%s", hostPath, tag)
}
