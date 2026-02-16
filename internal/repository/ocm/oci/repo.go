package oci

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

var ErrKeyChainNotNil = errors.New("keychain lookup must not be nil")

type (
	configFunc    func(string, ...crane.Option) ([]byte, error)
	pullLayerFunc func(string, ...crane.Option) (containerregistryv1.Layer, error)
)

// RepositoryReader provides basic support to read OCI data from OCI repositories.
type RepositoryReader struct {
	insecure       bool
	keyChainLookup spec.KeyChainLookup
	config         configFunc
	pullLayer      pullLayerFunc
}

func NewRepository(kcl spec.KeyChainLookup,
	insecure bool,
	opts ...func(*RepositoryReader) *RepositoryReader,
) (*RepositoryReader, error) {
	if kcl == nil {
		return nil, ErrKeyChainNotNil
	}

	repo := &RepositoryReader{
		insecure:       insecure,
		keyChainLookup: kcl,
		config:         crane.Config,
		pullLayer:      crane.PullLayer,
	}

	for _, opt := range opts {
		repo = opt(repo)
	}

	return repo, nil
}

// WithConfigFunction is a low level primitive that replaces the default crane.Config function.
func WithConfigFunction(f configFunc) func(*RepositoryReader) *RepositoryReader {
	return func(c *RepositoryReader) *RepositoryReader {
		c.config = f
		return c
	}
}

// WithPullLayerFunction is a low level primitive that replaces the default crane.PullLayer function.
func WithPullLayerFunction(f pullLayerFunc) func(*RepositoryReader) *RepositoryReader {
	return func(c *RepositoryReader) *RepositoryReader {
		c.pullLayer = f
		return c
	}
}

func (c *RepositoryReader) Config(ctx context.Context, ref string) ([]byte, error) {
	opts, err := c.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get crane options: %w", err)
	}

	return c.config(ref, opts...)
}

func (c *RepositoryReader) PullLayer(ctx context.Context, ref string) (containerregistryv1.Layer, error) {
	opts, err := c.stdOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get crane options: %w", err)
	}

	return c.pullLayer(ref, opts...)
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
