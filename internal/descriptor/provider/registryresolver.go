package provider

import (
	"net/url"
)

type RegistryResolver interface {
	GetOCIRegistrURL() (*url.URL, error)
}

type DefaultRegistryResolver struct {
	ociRegistryURL string
}

func NewDefaultRegistryResolver(ociRegistryURL string) *DefaultRegistryResolver {
	return &DefaultRegistryResolver{ociRegistryURL: ociRegistryURL}
}

func (d *DefaultRegistryResolver) GetOCIRegistrURL() (*url.URL, error) {
	return url.Parse(d.ociRegistryURL)
}
