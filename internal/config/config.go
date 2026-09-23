package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"go-unit-mangement/internal/auth"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	SessionTTL   time.Duration
	CookieSecure bool

	// Initial admin account, created on startup if no users exist yet.
	AdminUsername string
	AdminPassword string

	// OIDC enables SSO login when set (see OIDCEnabled).
	OIDC auth.OIDCConfig
	// OIDCDisplayName labels the SSO button on the login page.
	OIDCDisplayName string
}

func Load() (*Config, error) {
	ttl, err := time.ParseDuration(getEnv("SESSION_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_TTL: %w", err)
	}

	secure, err := strconv.ParseBool(getEnv("COOKIE_SECURE", "false"))
	if err != nil {
		return nil, fmt.Errorf("invalid COOKIE_SECURE: %w", err)
	}

	cfg := &Config{
		Addr:          getEnv("ADDR", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://app:app@localhost:5432/app?sslmode=disable"),
		SessionTTL:    ttl,
		CookieSecure:  secure,
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		OIDC: auth.OIDCConfig{
			IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
			ClientID:     os.Getenv("OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		},
		OIDCDisplayName: getEnv("OIDC_DISPLAY_NAME", "SSO"),
	}

	o := cfg.OIDC
	set := 0
	for _, v := range []string{o.IssuerURL, o.ClientID, o.ClientSecret, o.RedirectURL} {
		if v != "" {
			set++
		}
	}
	if set != 0 && set != 4 {
		return nil, errors.New("OIDC_ISSUER_URL, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET and OIDC_REDIRECT_URL must all be set to enable SSO")
	}
	if set == 4 {
		// A redirect URL pointing anywhere else lands the provider's
		// response on the single-page app, and sign-in silently never
		// completes.
		u, err := url.Parse(o.RedirectURL)
		if err != nil || !u.IsAbs() || u.Path != auth.OIDCCallbackPath {
			return nil, fmt.Errorf("OIDC_REDIRECT_URL must be <public URL>%s, got %q", auth.OIDCCallbackPath, o.RedirectURL)
		}
	}

	return cfg, nil
}

// OIDCEnabled reports whether SSO login is configured.
func (c *Config) OIDCEnabled() bool { return c.OIDC.IssuerURL != "" }

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
