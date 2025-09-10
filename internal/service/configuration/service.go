package configuration

import (
	"net/url"
)

type Service struct {
}

// GetOCIRegistryURL returns the OCI registry URL from the configuration.
func (s *Service) GetOCIRegistryURL() (*url.URL, error) {
	return nil, nil // TODO: implement
}
