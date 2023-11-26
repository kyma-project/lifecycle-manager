package watcher

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/jellydator/ttlcache/v3"
)

type CertificateCache struct {
	TTL time.Duration
	*ttlcache.Cache[string, *certmanagerv1.Certificate]
}

func NewCertificateCache(ttl time.Duration) *CertificateCache {
	cache := ttlcache.New[string, *certmanagerv1.Certificate]()
	go cache.Start()
	return &CertificateCache{Cache: cache, TTL: ttl}
}

func (c *CertificateCache) GetCACertFromCache(caCertName string) *certmanagerv1.Certificate {
	value := c.Cache.Get(caCertName)
	if value != nil {
		cert := value.Value()
		return cert
	}

	return nil
}

func (c *CertificateCache) SetCACertToCache(cert *certmanagerv1.Certificate) {
	c.Cache.Set(cert.Name, cert, c.TTL)
}
