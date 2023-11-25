package watcher

import (
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/jellydator/ttlcache/v3"
)

type CACertificateCache struct {
	TTL time.Duration
	*ttlcache.Cache[string, *certmanagerv1.Certificate]
}

func NewCACertificateCache(ttl time.Duration) *CACertificateCache {
	cache := ttlcache.New[string, *certmanagerv1.Certificate]()
	go cache.Start()
	return &CACertificateCache{Cache: cache, TTL: ttl}
}

func (c *CACertificateCache) GetCACertFromCache(caCertName string) *certmanagerv1.Certificate {
	value := c.Cache.Get(caCertName)
	if value != nil {
		cert := value.Value()
		return cert
	}

	return nil
}

func (c *CACertificateCache) SetCACertToCache(cert *certmanagerv1.Certificate) {
	c.Cache.Set(cert.Name, cert, c.TTL)
}
