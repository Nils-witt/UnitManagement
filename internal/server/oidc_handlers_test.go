package server

import "testing"

func TestSanitizeRedirectPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path, want string
	}{
		{"", "/"},
		{"/", "/"},
		{"/users", "/users"},
		{"/a/b?c=d", "/a/b?c=d"},
		{"users", "/"},
		{"//evil.example.com", "/"},
		{"https://evil.example.com/", "/"},
		{"/\\evil.example.com", "/"},
		{"/ok\r\nSet-Cookie: x", "/"},
	}
	for _, tc := range tests {
		if got := sanitizeRedirectPath(tc.path); got != tc.want {
			t.Errorf("sanitizeRedirectPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
