package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-unit-mangement/internal/auth"
)

const (
	oidcLoginPath    = "/api/auth/oidc/login"
	oidcCallbackPath = auth.OIDCCallbackPath

	// oidcFlowCookie carries the sign-in attempt's state, nonce, PKCE
	// verifier and post-login target from the login redirect to the callback.
	// It is scoped to the OIDC routes and lives only as long as a sign-in
	// reasonably takes.
	oidcFlowCookie     = "oidc_flow"
	oidcFlowCookiePath = "/api/auth/oidc"
	oidcFlowTTL        = 10 * time.Minute
)

// handleOIDCLogin starts SSO: GET /api/auth/oidc/login?redirect=/some/page.
func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	flow, err := auth.NewOIDCFlow()
	if err != nil {
		slog.Error("start oidc login", "err", err)
		redirectToLogin(w, r, "failed")
		return
	}

	v := url.Values{}
	v.Set("state", flow.State)
	v.Set("nonce", flow.Nonce)
	v.Set("verifier", flow.Verifier)
	v.Set("redirect", sanitizeRedirectPath(r.URL.Query().Get("redirect")))
	s.setOIDCFlowCookie(w, v.Encode(), int(oidcFlowTTL.Seconds()))

	http.Redirect(w, r, s.oidc.AuthCodeURL(flow), http.StatusFound)
}

// handleOIDCCallback finishes SSO: the provider redirects here with a code,
// which is exchanged for a verified identity and a normal session cookie.
// Failures go back to the login page with a short error code; details are
// only logged, since they may describe the provider's configuration.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	var flowValues url.Values
	if c, err := r.Cookie(oidcFlowCookie); err == nil {
		flowValues, _ = url.ParseQuery(c.Value)
	}
	// Single use: a flow must never be replayable, whatever happens next.
	s.setOIDCFlowCookie(w, "", -1)

	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		slog.Warn("oidc provider returned an error", "error", e, "description", q.Get("error_description"))
		redirectToLogin(w, r, "denied")
		return
	}
	if flowValues == nil {
		slog.Warn("oidc callback without a sign-in in progress")
		redirectToLogin(w, r, "expired")
		return
	}

	flow := auth.OIDCFlow{
		State:    flowValues.Get("state"),
		Nonce:    flowValues.Get("nonce"),
		Verifier: flowValues.Get("verifier"),
	}
	identity, err := s.oidc.Exchange(r.Context(), flow, q.Get("state"), q.Get("code"))
	if err != nil {
		slog.Warn("oidc callback", "err", err)
		redirectToLogin(w, r, "failed")
		return
	}

	token, user, expires, err := s.auth.LoginOIDC(r.Context(), identity)
	if err != nil {
		slog.Error("oidc login", "err", err)
		redirectToLogin(w, r, "failed")
		return
	}
	slog.Info("oidc login", "user", user.Username)

	s.setSessionCookie(w, token, expires)
	http.Redirect(w, r, sanitizeRedirectPath(flowValues.Get("redirect")), http.StatusFound)
}

func (s *Server) setOIDCFlowCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcFlowCookie,
		Value:    value,
		Path:     oidcFlowCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		// Lax, not Strict: the callback is a cross-site navigation from the
		// provider and must still carry this cookie.
		SameSite: http.SameSiteLaxMode,
	})
}

func redirectToLogin(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?sso_error="+url.QueryEscape(code), http.StatusFound)
}

// sanitizeRedirectPath returns path if it is a same-origin absolute path, and
// "/" otherwise, so the sign-in flow can't be used as an open redirect.
func sanitizeRedirectPath(path string) string {
	if path == "" || path[0] != '/' || strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "\\\r\n\t") {
		return "/"
	}
	return path
}
