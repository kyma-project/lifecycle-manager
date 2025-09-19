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
	ErrNoProtocolScheme = errors.New("hostPort must not contain protocol scheme (http/https)")
	ErrNoLeadingSlash   = errors.New("hostPort must not start with a '/'")
)

// RepositoryReader provides basic support to read data from OCI repositories.
type RepositoryReader struct {
	keyChainLookup spec.KeyChainLookup
	hostPort       string
	insecure       bool
	craneClient    CraneClient
}

func NewRepository(kcl spec.KeyChainLookup, hostPort string, insecure bool) (*RepositoryReader, error) {
	if !insecure && kcl == nil {
		return nil, ErrKeyChainNotNil
	}

	if strings.Contains(hostPort, "://") {
		return nil, fmt.Errorf("%w: %q", ErrNoProtocolScheme, hostPort)
	}

	if strings.HasPrefix(hostPort, "/") {
		return nil, fmt.Errorf("%w: %q", ErrNoLeadingSlash, hostPort)
	}

	return &RepositoryReader{
		keyChainLookup: kcl,
		hostPort:       hostPort,
		insecure:       insecure,
		craneClient:    &craneClient{},
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
	configBytes, err := s.craneClient.Config(ref, options...)
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
	configBytes, err := s.craneClient.PullLayer(refWithDigest, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ref=%q: %w", refWithDigest, err)
	}
	return configBytes, nil
}

func (s *RepositoryReader) HostRef() string {
	return s.hostPort
}

func (s *RepositoryReader) toImageRef(name, tag string) string {
	hostPath := path.Join(s.hostPort, "component-descriptors", name)
	return fmt.Sprintf("%s:%s", hostPath, tag)
}

func (s *RepositoryReader) stdOptions(ctx context.Context) ([]crane.Option, error) {
	options := []crane.Option{crane.WithContext(ctx)}
	if s.insecure {
		options = append(options, crane.Insecure)
	} else {
		keyChain, err := s.keyChainLookup.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get keychain: %w", err)
		}
		options = append(options, crane.WithAuthFromKeychain(keyChain))
	}
	return options, nil
}

// CraneClient defines the subset of crane functions used by RepositoryReader.
type CraneClient interface {
	Config(ref string, opt ...crane.Option) ([]byte, error)
	PullLayer(ref string, opt ...crane.Option) (containerregistryv1.Layer, error)
}

type craneClient struct{}

func (c *craneClient) Config(ref string, opt ...crane.Option) ([]byte, error) {
	return crane.Config(ref, opt...) //nolint:wrapcheck // the craneClient wrapper is ment not to wrap errors
}

func (c *craneClient) PullLayer(ref string, opt ...crane.Option) (containerregistryv1.Layer, error) {
	return crane.PullLayer(ref, opt...) //nolint:wrapcheck // the craneClient wrapper is ment not to wrap errors
}
