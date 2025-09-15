package credential

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/credentials/extensions/repositories/dockerconfig"
	"ocm.software/ocm/api/ocm/cpi"
)

var (
	ErrInvalidCredentialsFormat = errors.New("invalid credentials format, expected 'username:password'")
	errDockerConfigNotFound     = errors.New("docker config file not found in home directory")
)

func ResolveCredentials(ctx cpi.Context, userPasswordCreds, registryURL string) (credentials.Credentials, error) {
	if userPasswordCreds != "" {
		return resolveUserPasswordCredentials(userPasswordCreds)
	}

	creds, err := tryResolveFromDockerConfig(ctx, registryURL)
	if err == nil {
		return creds, nil
	}

	return credentials.NewCredentials(nil), nil
}

func resolveUserPasswordCredentials(userPasswordCreds string) (credentials.Credentials, error) {
	if validateCredFormat(userPasswordCreds) {
		return nil, ErrInvalidCredentialsFormat
	}

	user, pass := parseUserPass(userPasswordCreds)

	return credentials.DirectCredentials{
		"username": user,
		"password": pass,
	}, nil
}

// validateCredFormat checks if the credentials string is in the format "username:password", where both username and password contain at least one non-whitespace character
func validateCredFormat(creds string) bool {
	return !regexp.MustCompile(`^\S+:\S+$`).MatchString(creds)
}

func parseUserPass(credentials string) (string, string) {
	u, p, found := strings.Cut(credentials, ":")
	if !found {
		return "", ""
	}
	return u, p
}

func tryResolveFromDockerConfig(ctx cpi.Context, registryURL string) (credentials.Credentials, error) {
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".docker", "config.json")
		if repo, err := dockerconfig.NewRepository(ctx.CredentialsContext(), path, nil, true); err == nil {
			hostNameInDockerConfig := strings.Split(registryURL, "/")[0]
			creds, err := repo.LookupCredentials(hostNameInDockerConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to lookup credentials: %w", err)
			}
			return creds, nil
		}
	}

	return nil, errDockerConfigNotFound
}
