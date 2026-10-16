// Package config reads every setting from the environment, once, at boot.
package config

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/kthaisociety/business-crm/internal/auth"
)

type Config struct {
	// Host is the bind address. Empty binds all interfaces, which the container needs. Set it to
	// 127.0.0.1 for local dev: a dev machine with a public IP exposes anything bound to 0.0.0.0.
	Host string
	Port string

	// DatabaseURL is a Postgres connection string, e.g.
	// postgres://crm:crm@127.0.0.1:5436/crm?sslmode=disable.
	DatabaseURL string

	// DevUserEmail, when set, signs every request in as this user, for local work without an OIDC client
	// (docs/pcr/0007). Never set in a deployed environment.
	DevUserEmail string

	// Real sign-in (docs/adr/0004): the provider, this app's client there, where the app is served (its
	// callback is PUBLIC_URL/auth/callback), and the email domain that may sign in.
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	PublicURL        string
	SignInDomain     string

	// AdminEmails become admins when they sign in (docs/adr/0003), so a new environment has an admin.
	AdminEmails []string
}

func Load() *Config {
	return &Config{
		Host:         getEnv("HOST", ""),
		Port:         getEnv("PORT", "8000"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		DevUserEmail: strings.TrimSpace(getEnv("DEV_USER_EMAIL", "")),
		AdminEmails:  splitList(getEnv("ADMIN_EMAILS", "")),

		OIDCIssuer:       strings.TrimSpace(getEnv("OIDC_ISSUER", "https://accounts.google.com")),
		OIDCClientID:     strings.TrimSpace(getEnv("OIDC_CLIENT_ID", "")),
		OIDCClientSecret: strings.TrimSpace(getEnv("OIDC_CLIENT_SECRET", "")),
		PublicURL:        strings.TrimRight(strings.TrimSpace(getEnv("PUBLIC_URL", "")), "/"),
		SignInDomain:     strings.TrimSpace(getEnv("SIGN_IN_DOMAIN", "kthais.com")),
	}
}

// OIDC is the real sign-in's settings, or nil when no client is configured and the dev user (if any)
// signs requests in instead.
func (c *Config) OIDC() *auth.OIDCConfig {
	if c.OIDCClientID == "" && c.OIDCClientSecret == "" {
		return nil
	}
	return &auth.OIDCConfig{
		Issuer:       c.OIDCIssuer,
		ClientID:     c.OIDCClientID,
		ClientSecret: c.OIDCClientSecret,
		RedirectURL:  c.PublicURL + "/auth/callback",
		Domain:       c.SignInDomain,
		Google:       c.OIDCIssuer == "https://accounts.google.com",
	}
}

// DevUser is the user every request is signed in as, or nil when DEV_USER_EMAIL is unset.
func (c *Config) DevUser() *auth.User {
	if c.DevUserEmail == "" {
		return nil
	}
	return &auth.User{Subject: "dev", Email: c.DevUserEmail, Name: "Dev User"}
}

// Validate fails if a required setting is missing, naming every missing variable at once.
func (c *Config) Validate() error {
	var missing []string
	for name, value := range map[string]string{
		"DATABASE_URL": c.DatabaseURL,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if c.OIDC() != nil {
		for name, value := range map[string]string{
			"OIDC_ISSUER":        c.OIDCIssuer,
			"OIDC_CLIENT_ID":     c.OIDCClientID,
			"OIDC_CLIENT_SECRET": c.OIDCClientSecret,
			"PUBLIC_URL":         c.PublicURL,
			"SIGN_IN_DOMAIN":     c.SignInDomain,
		} {
			if value == "" {
				missing = append(missing, name)
			}
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if c.OIDC() != nil && c.DevUserEmail != "" {
		return fmt.Errorf("DEV_USER_EMAIL and an OIDC client are both set: unset DEV_USER_EMAIL to sign in for real (docs/pcr/0007)")
	}
	if c.OIDC() != nil && !strings.HasPrefix(c.PublicURL, "https://") && !isLoopback(c.PublicURL) {
		return fmt.Errorf("PUBLIC_URL must be https, or http on 127.0.0.1 or localhost for local work")
	}
	return nil
}

func isLoopback(url string) bool {
	return strings.HasPrefix(url, "http://127.0.0.1:") || strings.HasPrefix(url, "http://localhost:") ||
		url == "http://127.0.0.1" || url == "http://localhost"
}

// splitList reads a comma-separated list, dropping blanks.
func splitList(s string) []string {
	var items []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
