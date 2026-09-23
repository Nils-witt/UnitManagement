package auth

import (
	"strings"
	"testing"
)

func TestProvisionedUsername(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", MaxUsernameLength+10)
	tests := []struct {
		name string
		id   OIDCIdentity
		want string
	}{
		{"preferred username", OIDCIdentity{PreferredUsername: "jane", Email: "j@example.com", Subject: "s1"}, "jane"},
		{"falls back to email", OIDCIdentity{PreferredUsername: "  ", Email: "j@example.com", Subject: "s1"}, "j@example.com"},
		{"falls back to subject", OIDCIdentity{Subject: "s1"}, "s1"},
		{"skips a too long claim", OIDCIdentity{PreferredUsername: long, Email: "j@example.com"}, "j@example.com"},
		{"truncates a too long subject", OIDCIdentity{Subject: long}, long[:MaxUsernameLength]},
	}
	for _, tc := range tests {
		if got := provisionedUsername(tc.id); got != tc.want {
			t.Errorf("%s: provisionedUsername() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
