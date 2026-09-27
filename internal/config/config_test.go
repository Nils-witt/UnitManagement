package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLoadOIDCAccessTokens(t *testing.T) {
	sso := map[string]string{
		"OIDC_ISSUER_URL":    "https://idp.example",
		"OIDC_CLIENT_ID":     "client",
		"OIDC_CLIENT_SECRET": "secret",
		"OIDC_REDIRECT_URL":  "https://app.example/api/auth/oidc/callback",
	}
	tests := []struct {
		name          string
		env           map[string]string
		sso           bool
		wantErr       bool
		wantAudiences []string
	}{
		{"disabled", nil, false, false, nil},
		{"audience with sso", map[string]string{"OIDC_ACCESS_TOKEN_AUDIENCE": " api , ,client"}, true, false, []string{"api", "client"}},
		{"audience with extra issuer", map[string]string{"OIDC_ACCESS_TOKEN_AUDIENCE": "api", "OIDC_ACCESS_TOKEN_ISSUERS": "https://idp.example"}, false, false, []string{"api"}},
		{"audience without issuer", map[string]string{"OIDC_ACCESS_TOKEN_AUDIENCE": "api"}, false, true, nil},
		{"issuer without audience", map[string]string{"OIDC_ACCESS_TOKEN_ISSUERS": "https://idp.example"}, true, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"OIDC_ISSUER_URL", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OIDC_REDIRECT_URL", "OIDC_ACCESS_TOKEN_AUDIENCE", "OIDC_ACCESS_TOKEN_ISSUERS"} {
				t.Setenv(k, "")
			}
			if tc.sso {
				for k, v := range sso {
					t.Setenv(k, v)
				}
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %t", err, tc.wantErr)
			}
			if err == nil && !slices.Equal(cfg.OIDC.AccessTokenAudiences, tc.wantAudiences) {
				t.Errorf("audiences = %q, want %q", cfg.OIDC.AccessTokenAudiences, tc.wantAudiences)
			}
		})
	}
}

func TestLoadAdminPasswordLength(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", strings.Repeat("x", 72))
	if _, err := Load(); err != nil {
		t.Fatalf("72 bytes: err = %v, want nil", err)
	}
	t.Setenv("ADMIN_PASSWORD", strings.Repeat("x", 73))
	if _, err := Load(); err == nil {
		t.Fatal("73 bytes: err = nil, want error")
	}
}

func TestLoadPositionHistoryRetention(t *testing.T) {
	tests := []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"720h", 720 * time.Hour, false},
		{"-1h", 0, true},
		{"soon", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("POSITION_HISTORY_RETENTION", tc.value)
			cfg, err := Load()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %t", err, tc.wantErr)
			}
			if err == nil && cfg.PositionHistoryRetention != tc.want {
				t.Errorf("retention = %s, want %s", cfg.PositionHistoryRetention, tc.want)
			}
		})
	}
}
