package credential_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ocm.software/ocm/api/ocm/cpi"

	"github.com/kyma-project/lifecycle-manager/internal/repository/oci/credential"
)

func TestResolveCredentials_WhenCalledWithInvalidUsernamePasswordFormats_ReturnsError(t *testing.T) {
	_, err := credential.ResolveCredentials(nil, "invalidFormat", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, ":", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, ": ", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, " :", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, " : ", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, "user :pass", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)

	_, err = credential.ResolveCredentials(nil, "user:pass ", "")
	require.ErrorIs(t, err, credential.ErrInvalidCredentialsFormat)
}

func TestResolveCredentials_ReturnUserPasswordWhenGiven(t *testing.T) {
	userPasswordCreds := "user1:pass1"
	creds, err := credential.ResolveCredentials(nil, userPasswordCreds, "")

	require.NoError(t, err)
	require.Equal(t, "user1", creds.GetProperty("username"))
	require.Equal(t, "pass1", creds.GetProperty("password"))
}

func TestResolveCredentials_WhenNoCredentialsPassedAndNoDockerConfig_ReturnsEmptyCredentials(t *testing.T) {
	creds, err := credential.ResolveCredentials(cpi.DefaultContext(), "", "ghcr.io/template-operator")

	require.NoError(t, err)
	require.Empty(t, creds.GetProperty("username"))
	require.Empty(t, creds.GetProperty("password"))
}
