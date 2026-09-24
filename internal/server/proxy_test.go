package server

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRealIP(t *testing.T) {
	t.Parallel()

	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("::1/128")}

	tests := []struct {
		name    string
		remote  string
		headers map[string][]string
		want    string
	}{
		{"no headers", "10.0.0.1:1234", nil, "10.0.0.1:1234"},
		{"untrusted peer ignores headers", "203.0.113.9:1234", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}}, "203.0.113.9:1234"},
		{"single hop", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}}, "198.51.100.1"},
		{"spoofed entry left of client", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"1.2.3.4, 198.51.100.1"}}, "198.51.100.1"},
		{"skips trusted hops", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"198.51.100.1, 10.0.0.2", "10.0.0.3"}}, "198.51.100.1"},
		{"all hops trusted", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"10.0.0.3, 10.0.0.2"}}, "10.0.0.3"},
		{"stops at malformed entry", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"198.51.100.1, garbage, 10.0.0.2"}}, "10.0.0.2"},
		{"malformed only", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"garbage"}}, "10.0.0.1:1234"},
		{"entry with port", "10.0.0.1:1234", map[string][]string{"X-Forwarded-For": {"[2001:db8::1]:443"}}, "2001:db8::1"},
		{"x-real-ip", "[::1]:1234", map[string][]string{"X-Real-Ip": {"198.51.100.1"}}, "198.51.100.1"},
		{"ipv4-mapped peer", "[::ffff:10.0.0.1]:1234", map[string][]string{"X-Forwarded-For": {"198.51.100.1"}}, "198.51.100.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			h := realIP(trusted, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for k, v := range tt.headers {
				req.Header[k] = v
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tt.want)
			}
		})
	}
}
