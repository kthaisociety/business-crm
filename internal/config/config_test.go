package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
}

func TestNoDevUserByDefault(t *testing.T) {
	setRequired(t)
	t.Setenv("DEV_USER_EMAIL", "")

	assert.Nil(t, Load().DevUser())
}

func TestDevUserFromEnv(t *testing.T) {
	setRequired(t)
	t.Setenv("DEV_USER_EMAIL", " dev@kthais.com ")

	u := Load().DevUser()
	require.NotNil(t, u)
	assert.Equal(t, "dev@kthais.com", u.Email)
}

func setOIDC(t *testing.T) {
	t.Setenv("OIDC_CLIENT_ID", "client")
	t.Setenv("OIDC_CLIENT_SECRET", "secret")
	t.Setenv("PUBLIC_URL", "https://crm.example.com/")
	t.Setenv("DEV_USER_EMAIL", "")
}

func TestNoOIDCWithoutAClient(t *testing.T) {
	setRequired(t)

	assert.Nil(t, Load().OIDC())
}

func TestOIDCFromEnv(t *testing.T) {
	setRequired(t)
	setOIDC(t)

	cfg := Load()
	require.NoError(t, cfg.Validate())
	o := cfg.OIDC()
	require.NotNil(t, o)
	assert.Equal(t, "https://accounts.google.com", o.Issuer)
	assert.Equal(t, "https://crm.example.com/auth/callback", o.RedirectURL)
	assert.Equal(t, "kthais.com", o.Domain)
	assert.True(t, o.Google)
}

func TestOIDCNeedsTheWholeClientAndAPublicURL(t *testing.T) {
	setRequired(t)
	t.Setenv("OIDC_CLIENT_ID", "client")

	err := Load().Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OIDC_CLIENT_SECRET, PUBLIC_URL")
}

func TestOIDCAndTheDevUserDontMix(t *testing.T) {
	setRequired(t)
	setOIDC(t)
	t.Setenv("DEV_USER_EMAIL", "dev@kthais.com")

	assert.ErrorContains(t, Load().Validate(), "DEV_USER_EMAIL")
}

func TestOIDCNeedsHTTPSExceptOnThisMachine(t *testing.T) {
	setRequired(t)
	setOIDC(t)
	t.Setenv("PUBLIC_URL", "http://crm.example.com")
	assert.ErrorContains(t, Load().Validate(), "PUBLIC_URL must be https")

	t.Setenv("PUBLIC_URL", "http://127.0.0.1:5174")
	assert.NoError(t, Load().Validate())
}
