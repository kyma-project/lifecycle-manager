package certificate_test

import (
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/tests/fixtures/certificates"
)

var (
	cert1       = certificates.Cert1
	cert2       = certificates.Cert2
	cert3       = certificates.Cert3
	certExpired = certificates.CertExpired
)

func Test_AddCert_AddsCert(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert3, cert2, cert1)

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert3)

	require.NoError(t, err)
	assert.True(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_AddCert_AddsCertToEmptyBundle(t *testing.T) {
	bundle := []byte{}
	expectedBundle := cert1

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert1)

	require.NoError(t, err)
	assert.True(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_AddCert_AddsCertToNilBundle(t *testing.T) {
	var bundle []byte
	expectedBundle := cert1

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert1)

	require.NoError(t, err)
	assert.True(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_AddCert_NoOpOnExistingCert(t *testing.T) {
	bundle := appendCerts(cert3, cert2, cert1)
	expectedBundle := appendCerts(cert3, cert2, cert1)

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert3)

	require.NoError(t, err)
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_Bundle_NoOpOnEmptyCert(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, []byte(""))

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	require.Contains(t, err.Error(), "newCert")
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_Bundle_NoOpOnNilCert(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, nil)

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	require.Contains(t, err.Error(), "newCert")
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_Bundle_NoOpOnInvalidCert(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, []byte("invalid cert"))

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	require.Contains(t, err.Error(), "newCert")
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_Bundle_NoOpOnInvalidBundle(t *testing.T) {
	bundle := []byte("invalid bundle")
	expectedBundle := []byte("invalid bundle")

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert1)

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	require.Contains(t, err.Error(), "bundle")
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_Bundle_NoOpOnBundleWithInvalidParts(t *testing.T) {
	bundle := appendCerts(cert1, []byte("invalid string"))
	expectedBundle := appendCerts(cert1, []byte("invalid string"))

	bndlr := certificate.NewBundler()

	added, err := bndlr.Bundle(&bundle, cert1)

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	require.Contains(t, err.Error(), "bundle")
	assert.False(t, added)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_DropsExpiredCerts(t *testing.T) {
	bundle := appendCerts(cert2, certExpired, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler()

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.NoError(t, err)
	assert.True(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_NoOpOnNoExpiredCerts(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler()

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.NoError(t, err)
	assert.False(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_NoOpOnEmptyBundle(t *testing.T) {
	bundle := []byte{}
	expectedBundle := []byte{}

	bndlr := certificate.NewBundler()

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.NoError(t, err)
	assert.False(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_NoOpOnNilBundle(t *testing.T) {
	var bundle []byte

	bndlr := certificate.NewBundler()

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.NoError(t, err)
	assert.False(t, dropped)
	assert.Nil(t, bundle)
}

func Test_DropExpiredCerts_NoOpOnInvalidBundle(t *testing.T) {
	bundle := appendCerts(cert2, []byte("invalid string"))
	expectedBundle := appendCerts(cert2, []byte("invalid string"))

	bndlr := certificate.NewBundler()

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	assert.False(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_NoOpOnErrorParsingX509(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler(
		certificate.WithParseX509Function(
			func(_ []byte) (*x509.Certificate, error) {
				return nil, errors.New("error parsing x509")
			},
		),
	)

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.ErrorIs(t, err, certificate.ErrFailedToParseX509)
	assert.False(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func Test_DropExpiredCerts_NoOpOnEmptyNotBefore(t *testing.T) {
	bundle := appendCerts(cert2, cert1)
	expectedBundle := appendCerts(cert2, cert1)

	bndlr := certificate.NewBundler(
		certificate.WithParseX509Function(
			func(_ []byte) (*x509.Certificate, error) {
				return &x509.Certificate{
					NotAfter: time.Time{},
				}, nil
			},
		),
	)

	dropped, err := bndlr.DropExpiredCerts(&bundle)

	require.ErrorIs(t, err, certificate.ErrX509NotAfterIsZero)
	assert.False(t, dropped)
	assert.Equal(t, expectedBundle, bundle)
}

func appendCerts(certs ...[]byte) []byte {
	capHint := 0
	for _, cert := range certs {
		capHint += len(cert)
	}
	appended := make([]byte, 0, capHint)
	for _, cert := range certs {
		appended = append(appended, cert...)
	}
	return appended
}
