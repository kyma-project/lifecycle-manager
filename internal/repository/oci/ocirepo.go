package oci

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

var (
	ErrKeyChainNotNil   = errors.New("keychain lookup must not be nil")
	ErrNoProtocolScheme = errors.New("hostref must not contain protocol scheme (http/https)")
	ErrNoLeadingSlash   = errors.New("hostref must not start with a '/'")
)

// RepositoryReader provides basic support to read data from OCI repositories.
type RepositoryReader struct {
	keyChainLookup spec.KeyChainLookup
	hostref        string
	insecure       bool
	cWrapper       craneWrapper // in runtime delegates to crane package functions
}

// NewRepository creates a new RepositoryReader for the given hostref.
// If insecure is false, a non-nil KeyChainLookup must be provided to retrieve authentication information.
// The host must not contain a protocol scheme (http/https), for example: "k3d-kcp-registry.localhost:5000".
func NewRepository(kcl spec.KeyChainLookup, hostWithPort string, insecure bool,
	cWrapper craneWrapper,
) (*RepositoryReader, error) {
	if !insecure && kcl == nil {
		return nil, ErrKeyChainNotNil
	}

	if strings.Contains(hostWithPort, "://") {
		return nil, fmt.Errorf("%w: %q", ErrNoProtocolScheme, hostWithPort)
	}

	if strings.HasPrefix(hostWithPort, "/") {
		return nil, fmt.Errorf("%w: %q", ErrNoLeadingSlash, hostWithPort)
	}

	return &RepositoryReader{
		keyChainLookup: kcl,
		hostref:        hostWithPort,
		insecure:       insecure,
		cWrapper:       cWrapper,
	}, nil
}

// GetConfigFile retrieves the OCI artifact config file as a byte slice.
// We're not using image-oriented types here because OCM artifacts "config file" is not a standard image config.
func (s *RepositoryReader) GetConfigFile(ctx context.Context, name, tag string) ([]byte, error) {
	options, err := s.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get standard options: %w", err)
	}
	ref := s.toImageRef(name, tag)
	configBytes, err := s.cWrapper.Config(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for ref=%q: %w", ref, err)
	}

	return configBytes, nil
}

// PullLayer retrieves a layer with given digest from an OCI artifact identified by name and tag.
func (s *RepositoryReader) PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error) {
	options, err := s.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get standard options: %w", err)
	}
	ref := s.toImageRef(name, tag)
	refWithDigest := fmt.Sprintf("%s@%s", ref, digest)
	configBytes, err := s.cWrapper.PullLayer(refWithDigest, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ref=%q: %w", refWithDigest, err)
	}
	return configBytes, nil
}

func (s *RepositoryReader) toImageRef(name, tag string) string {
	hostPath := path.Join(s.hostref, "component-descriptors", name)
	return fmt.Sprintf("%s:%s", hostPath, tag)
}

func (s *RepositoryReader) stdOptions(ctx context.Context) ([]crane.Option, error) {
	options := []crane.Option{crane.WithContext(ctx)}

	keyChain, err := s.keyChainLookup.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get keychain: %w", err)
	}
	options = append(options, crane.WithAuthFromKeychain(keyChain))

	if s.insecure {
		options = append(options, crane.Insecure)
	}

	return options, nil
}

// craneWrapper is a subset of crane package functions used by RepositoryReader.
// It is introduced to facilitate testing.
type craneWrapper interface {
	Config(ref string, opt ...crane.Option) ([]byte, error)
	PullLayer(ref string, opt ...crane.Option) (containerregistryv1.Layer, error)
}

type DefaultCraneWrapper struct{}

func (c *DefaultCraneWrapper) Config(ref string, opt ...crane.Option) ([]byte, error) {
	return crane.Config(ref, opt...) //nolint:wrapcheck // the crane wrapper should be transparent
}

func (c *DefaultCraneWrapper) PullLayer(ref string, opt ...crane.Option) (containerregistryv1.Layer, error) {
	return crane.PullLayer(ref, opt...) //nolint:wrapcheck // the crane wrapper should be transparent
}
