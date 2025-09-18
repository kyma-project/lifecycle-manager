package oci

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

// RepositoryReader provides basic support to read data from OCI repositories.
type RepositoryReader struct {
	keyChain spec.KeyChainLookup
	hostPort string
	insecure bool
}

func NewRepository(kcl spec.KeyChainLookup, hostPort string, insecure bool) (*RepositoryReader, error) {

	if kcl == nil {
		return nil, fmt.Errorf("keychain lookup must not be nil")
	}

	if strings.Contains(hostPort, "://") {
		return nil, fmt.Errorf("hostPort must not contain protocol scheme (http/https): %q", hostPort)
	}

	if strings.HasPrefix(hostPort, "/") {
		return nil, fmt.Errorf("hostPort must not start with a '/': %q", hostPort)
	}

	return &RepositoryReader{
		keyChain: kcl,
		hostPort: hostPort,
		insecure: insecure,
	}, nil
}

func (s *RepositoryReader) stdOptions(ctx context.Context) ([]crane.Option, error) {

	var options []crane.Option = []crane.Option{crane.WithContext(ctx)}
	if s.insecure {
		options = append(options, crane.Insecure)
	} else {
		keyChain, err := s.keyChain.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get keychain: %w", err)
		}
		options = append(options, crane.WithAuthFromKeychain(keyChain))
	}
	return options, nil
}

// GetConfigFile retrieves the OCI artifact config file as a byte slice.
// We're not using image-oriented types here because OCM artifacts "config file" is not a standard image config.
func (s *RepositoryReader) GetConfigFile(ctx context.Context, name, tag string) ([]byte, error) {

	options, err := s.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get standard options: %w", err)
	}
	ref := s.toRef(name, tag)
	configBytes, err := crane.Config(ref, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to get config file for ref=%q: %w", ref, err)
	}

	return configBytes, nil
}

func (s *RepositoryReader) PullLayer(ctx context.Context, name, tag, digest string) (v1.Layer, error) {
	options, err := s.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get standard options: %w", err)
	}
	ref := s.toRef(name, tag)
	refWithDigest := fmt.Sprintf("%s@%s", ref, digest)
	configBytes, err := crane.PullLayer(refWithDigest, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ref=%q: %w", refWithDigest, err)
	}
	return configBytes, nil
}

func (s *RepositoryReader) HostRef() string {
	return s.hostPort
}

func (s *RepositoryReader) toRef(name, tag string) string {
	hostPath := path.Join(s.hostPort, "component-descriptors", name)
	return fmt.Sprintf("%s:%s", hostPath, tag)
}
