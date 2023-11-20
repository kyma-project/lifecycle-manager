package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (c *CertificateManager) GetCACertificate(ctx context.Context) (*certmanagerv1.Certificate, error) {
	cachedCert := c.GetCACertFromCache()

	// If Cache is empty or Renewal Time has been passed, then renew Cache
	if cachedCert == nil || cachedCert.Status.RenewalTime.Before(&(apimetav1.Time{Time: time.Now()})) {
		caCert := &certmanagerv1.Certificate{}
		if err := c.kcpClient.Get(ctx, client.ObjectKey{Namespace: c.istioNamespace, Name: c.caCertName}, caCert); err != nil {
			return nil, fmt.Errorf("failed to get CA certificate %w", err)
		}
		c.SetCACertToCache(caCert)
		return caCert, nil
	}

	return cachedCert, nil
}
