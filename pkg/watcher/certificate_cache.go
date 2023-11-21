package watcher

import (
	"sync"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

//nolint:gochecknoglobals
var certificateCache = sync.Map{}

func (c *CertificateManager) GetCACertFromCache() *certmanagerv1.Certificate {
	value, ok := certificateCache.Load(c.caCertName)
	if !ok {
		return nil
	}
	cert, ok := value.(*certmanagerv1.Certificate)
	if !ok {
		return nil
	}

	return cert
}

func (c *CertificateManager) SetCACertToCache(cert *certmanagerv1.Certificate) {
	certificateCache.Store(c.caCertName, cert)
}
