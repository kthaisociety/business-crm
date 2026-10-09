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
